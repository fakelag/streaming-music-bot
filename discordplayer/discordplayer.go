package discordplayer

import (
	"errors"
	"musicbot/entities"
	"sync"

	. "musicbot/discordplayer/interfaces"

	"github.com/bwmarrin/discordgo"
)

type DiscordMusicSession struct {
	mutex sync.RWMutex

	// Static fields, unlocked access
	guildID        string
	voiceChannelID string
	discordSession DiscordSession

	// Worker fields, unlocked access in worker goroutine
	dca             DiscordAudio
	voiceConnection DiscordVoiceConnection

	mediaQueue        []entities.Media
	mediaQueueMaxSize int

	workerActive     bool
	chanLeaveCommand chan bool
}

type DiscordMusicSessionOptions struct {
	GuildID           string
	VoiceChannelID    string
	MediaQueueMaxSize int // Default 100
}

func NewDiscordMusicSession(
	discord *discordgo.Session,
	options *DiscordMusicSessionOptions,
) (*DiscordMusicSession, error) {
	return NewDiscordMusicSessionEx(
		NewDiscordAudio(),
		NewDiscordSession(discord),
		options,
	)
}

func NewDiscordMusicSessionEx(
	dca DiscordAudio,
	discord DiscordSession,
	options *DiscordMusicSessionOptions,
) (*DiscordMusicSession, error) {
	queueMaxSize := options.MediaQueueMaxSize

	if queueMaxSize == 0 {
		queueMaxSize = 100
	}

	dms := &DiscordMusicSession{
		guildID:           options.GuildID,
		voiceChannelID:    options.VoiceChannelID,
		voiceConnection:   nil,
		discordSession:    discord,
		dca:               dca,
		mediaQueue:        make([]entities.Media, 0),
		mediaQueueMaxSize: options.MediaQueueMaxSize,
		chanLeaveCommand:  make(chan bool, 1),
	}

	return dms, nil
}

func (dms *DiscordMusicSession) EnqueueMedia(media entities.Media) error {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	if len(dms.mediaQueue) == dms.mediaQueueMaxSize {
		return errors.New("queue full")
	}

	dms.mediaQueue = append(dms.mediaQueue, media)

	if !dms.workerActive {
		go dms.voiceWorker()
		dms.workerActive = true
	}

	return nil
}

func (dms *DiscordMusicSession) StartPlaylist(playlist entities.Playlist) {
}

func (dms *DiscordMusicSession) ClearPlaylist() {
}

func (dms *DiscordMusicSession) Repeat() {
}

func (dms *DiscordMusicSession) Skip() {
}

func (dms *DiscordMusicSession) Leave() bool {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	if dms.workerActive && len(dms.chanLeaveCommand) == 0 {
		dms.chanLeaveCommand <- true
		return true
	}
	return false
}
