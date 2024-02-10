package discordplayer

import (
	"errors"
	"musicbot/entities"
	"sync"
	"time"

	. "musicbot/discordplayer/interfaces"

	"github.com/bwmarrin/discordgo"
)

var (
	ErrorWorkerNotActive    = errors.New("voice worker inactive")
	ErrorNotStreaming       = errors.New("not currently streaming")
	ErrorCommandAlreadySent = errors.New("command already sent")
	ErrorInvalidMedia       = errors.New("invalid media")
	ErrorMediaQueueFull     = errors.New("media queue full")
	ErrorNoMediaToRepeat    = errors.New("no media to repeat")
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

	if media == nil {
		return ErrorInvalidMedia
	}

	if len(dms.mediaQueue) == dms.mediaQueueMaxSize {
		return ErrorMediaQueueFull
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

func (dms *DiscordMusicSession) Repeat() error {
	err := dms.sendCommand(dms.chanRepeatCommand)

	if err == nil {
		return nil
	}

	if errors.Is(err, ErrorWorkerNotActive) {
		dms.mutex.RLock()
		lastCompletedMedia := dms.lastCompletedMedia
		dms.mutex.RUnlock()

		if lastCompletedMedia == nil {
			return ErrorNoMediaToRepeat
		}

		return dms.EnqueueMedia(lastCompletedMedia)
	}

	return err
}

func (dms *DiscordMusicSession) SetPaused(paused bool) error {
	dms.mutex.RLock()
	defer dms.mutex.RUnlock()

	if dms.currentMediaSession == nil {
		return ErrorNotStreaming
	}

	dms.currentMediaSession.streamingSession.SetPaused(paused)
	return nil
}

func (dms *DiscordMusicSession) IsPaused() (bool, error) {
	dms.mutex.RLock()
	defer dms.mutex.RUnlock()

	if dms.currentMediaSession == nil {
		return false, ErrorNotStreaming
	}

	isPaused := dms.currentMediaSession.streamingSession.Paused()
	return isPaused, nil
}

func (dms *DiscordMusicSession) Skip() error {
	return dms.sendCommand(dms.chanSkipCommand)
}

func (dms *DiscordMusicSession) Leave() error {
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

func (dms *DiscordMusicSession) sendCommand(command chan bool) error {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	if !dms.workerActive {
		return ErrorWorkerNotActive
	}

	if len(command) > 0 {
		return ErrorCommandAlreadySent
	}

	command <- true
	return nil
}
