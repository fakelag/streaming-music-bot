package discordplayer

import (
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

	dca DiscordAudio

	mediaQueue      []entities.Media
	voiceConnection DiscordVoiceConnection

	workerActive     bool
	chanLeaveCommand chan bool
}

func NewDiscordMusicSession(
	discord *discordgo.Session,
	guildId string,
	voiceChannelID string,
) (*DiscordMusicSession, error) {
	return NewDiscordMusicSessionEx(
		NewDiscordAudio(),
		NewDiscordSession(discord),
		guildId,
		voiceChannelID,
	)
}

func NewDiscordMusicSessionEx(
	dca DiscordAudio,
	discord DiscordSession,
	guildID string,
	voiceChannelID string,
) (*DiscordMusicSession, error) {
	dms := &DiscordMusicSession{
		guildID:          guildID,
		voiceChannelID:   voiceChannelID,
		voiceConnection:  nil,
		discordSession:   discord,
		dca:              dca,
		mediaQueue:       make([]entities.Media, 0),
		chanLeaveCommand: make(chan bool, 1),
	}

	return dms, nil
}

func (dms *DiscordMusicSession) EnqueueMedia(media entities.Media) {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	dms.mediaQueue = append(dms.mediaQueue, media)

	if !dms.workerActive {
		go dms.voiceWorker()
	}
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
