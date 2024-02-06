package discordplayer

import (
	"errors"
	"musicbot/entities"
	"sync"
	"time"

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

	currentMediaSession   *DcaMediaSession
	currentlyPlayingMedia entities.Media
	lastCompletedMedia    entities.Media
	mediaQueue            []entities.Media
	mediaQueueMaxSize     int

	workerActive      bool
	chanLeaveCommand  chan bool
	chanSkipCommand   chan bool
	chanRepeatCommand chan bool
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
		chanSkipCommand:   make(chan bool, 1),
		chanRepeatCommand: make(chan bool, 1),
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

func (dms *DiscordMusicSession) ClearMediaQueue() bool {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	if len(dms.mediaQueue) == 0 {
		return false
	}

	dms.mediaQueue = make([]entities.Media, 0)
	return true
}

func (dms *DiscordMusicSession) Repeat() (repeatAlreadySet bool, err error) {
	dms.mutex.Lock()

	if dms.workerActive {
		if len(dms.chanRepeatCommand) == 0 {
			dms.chanRepeatCommand <- true
			dms.mutex.Unlock()
			return false, nil
		}

		// Repeat command already sent
		dms.mutex.Unlock()
		return true, nil
	}

	dms.mutex.Unlock()
	err = dms.EnqueueMedia(dms.lastCompletedMedia)
	return false, err
}

func (dms *DiscordMusicSession) Skip() bool {
	return dms.sendCommand(dms.chanSkipCommand)
}

func (dms *DiscordMusicSession) Leave() bool {
	return dms.sendCommand(dms.chanLeaveCommand)
}

func (dms *DiscordMusicSession) GetMediaQueue() []entities.Media {
	dms.mutex.RLock()
	defer dms.mutex.RUnlock()

	mediaQueueCopy := make([]entities.Media, len(dms.mediaQueue))
	copy(mediaQueueCopy, dms.mediaQueue)

	return mediaQueueCopy
}

func (dms *DiscordMusicSession) GetCurrentlyPlayingMedia() entities.Media {
	dms.mutex.RLock()
	defer dms.mutex.RUnlock()
	return dms.currentlyPlayingMedia
}

func (dms *DiscordMusicSession) CurrentPlaybackPosition() time.Duration {
	dms.mutex.RLock()
	defer dms.mutex.RUnlock()

	if dms.currentMediaSession == nil {
		return time.Duration(0)
	}

	return dms.currentMediaSession.streamingSession.PlaybackPosition()
}

func (dms *DiscordMusicSession) sendCommand(command chan bool) bool {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	if dms.workerActive && len(command) == 0 {
		command <- true
		return true
	}
	return false
}
