package discordplayer

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/fakelag/streaming-music-bot/entities"

	. "github.com/fakelag/streaming-music-bot/discordplayer/interfaces"

	"github.com/bwmarrin/discordgo"
)

var (
	ErrorWorkerNotActive         = errors.New("voice worker inactive")
	ErrorWorkerAlreadyActive     = errors.New("voice worker is already active")
	ErrorNotStreaming            = errors.New("not currently streaming")
	ErrorCommandAlreadySent      = errors.New("command already sent")
	ErrorInvalidMedia            = errors.New("invalid media")
	ErrorMediaQueueFull          = errors.New("media queue full")
	ErrorNoMediaFound            = errors.New("no media found")
	ErrorMediaUnsupportedFeature = errors.New("unsupported feature by current media")
	ErrorInvalidArgument         = errors.New("invalid argument")
	ErrorNoVoiceChannelSet       = errors.New("no voice channel set")
	ErrorWaitingForWorkerTimeout = errors.New("timed out waiting for worker")
)

type NextMediaCallback = func(session *DiscordMusicSession, mediaFile entities.Media, isReload bool)
type ErrorCallback = func(session *DiscordMusicSession, mediaFile entities.Media, err error)

type DiscordMusicSession struct {
	mutex sync.RWMutex

	// Static fields, unlocked access
	guildID                            string
	discordSession                     DiscordSession
	leaveAfterChannelEmptyTime         time.Duration
	leaveAfterChannelEmptyTimeInterval time.Duration
	parentCtx                          context.Context

	// Worker fields, unlocked access in worker goroutine
	dca             DiscordAudio
	voiceConnection DiscordVoiceConnection
	workerCtx       context.Context

	voiceChannelID        string
	currentMediaSession   *DcaMediaSession
	currentlyPlayingMedia entities.Media
	lastCompletedMedia    entities.Media
	mediaQueue            []entities.Media
	mediaQueueMaxSize     int
	currentPlaylist       entities.Playlist

	nextMediaCallbacks []NextMediaCallback
	errorCallbacks     []ErrorCallback

	workerActive      bool
	chanLeaveCommand  chan bool
	chanSkipCommand   chan bool
	chanReplayCommand chan bool
	chanJumpCommand   chan time.Duration
}

type DiscordMusicSessionOptions struct {
	GuildID        string
	VoiceChannelID string
	// Max number of media that is able to be queued up. Default 100
	// Does not apply to playlists
	MediaQueueMaxSize int
	// Number of time before automatically exiting if the current voice channel
	// is empty. Members are checked every 10 seconds. Pass 0 to stay forever. Defaults to 0
	LeaveAfterChannelEmptyTime time.Duration
}

func NewDiscordMusicSession(
	ctx context.Context,
	discord *discordgo.Session,
	options *DiscordMusicSessionOptions,
) (*DiscordMusicSession, error) {
	return NewDiscordMusicSessionEx(
		ctx,
		NewDiscordAudio(),
		NewDiscordSession(discord),
		10*time.Second,
		options,
	)
}

func NewDiscordMusicSessionEx(
	ctx context.Context,
	dca DiscordAudio,
	discord DiscordSession,
	checkChannelEmptyInterval time.Duration,
	options *DiscordMusicSessionOptions,
) (*DiscordMusicSession, error) {
	queueMaxSize := options.MediaQueueMaxSize

	if queueMaxSize == 0 {
		queueMaxSize = 100
	}

	dms := &DiscordMusicSession{
		guildID:                            options.GuildID,
		voiceChannelID:                     options.VoiceChannelID,
		voiceConnection:                    nil,
		leaveAfterChannelEmptyTime:         options.LeaveAfterChannelEmptyTime,
		leaveAfterChannelEmptyTimeInterval: checkChannelEmptyInterval,
		discordSession:                     discord,
		dca:                                dca,
		workerCtx:                          nil,
		parentCtx:                          ctx,
		mediaQueue:                         make([]entities.Media, 0),
		mediaQueueMaxSize:                  options.MediaQueueMaxSize,
		nextMediaCallbacks:                 make([]NextMediaCallback, 0),
		errorCallbacks:                     make([]ErrorCallback, 0),
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
	return nil
}

func (dms *DiscordMusicSession) SetPlaylist(playlist entities.Playlist) {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()
	dms.currentPlaylist = playlist
}

// Starts voice worker, returns an error if the worker is already active
func (dms *DiscordMusicSession) Start() (context.Context, error) {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	if dms.workerActive {
		return nil, ErrorWorkerAlreadyActive
	}

	if dms.voiceChannelID == "" {
		return nil, ErrorNoVoiceChannelSet
	}

	dms.chanLeaveCommand = make(chan bool, 1)
	dms.chanSkipCommand = make(chan bool, 1)
	dms.chanReplayCommand = make(chan bool, 1)
	dms.chanJumpCommand = make(chan time.Duration, 1)

	workerCtx, cancel := context.WithCancel(dms.parentCtx)
	dms.workerCtx = workerCtx

	go dms.voiceWorker(cancel)
	dms.workerActive = true

	return workerCtx, nil
}

// Noop if current voice channel is already set to the given voiceChannelID
// If current voice channel is different, this stops the worker and sets new channel id
func (dms *DiscordMusicSession) SetVoiceChannelID(voiceChannelID string) (workerStopped bool, err error) {
	dms.mutex.RLock()
	if dms.voiceChannelID == voiceChannelID {
		dms.mutex.RUnlock()
		return false, nil
	}
	isWorkerActive := dms.workerActive
	workerCtx := dms.workerCtx
	dms.mutex.RUnlock()

	if isWorkerActive {
		dms.Leave()

		select {
		case <-workerCtx.Done():
			break
		case <-time.After(5 * time.Second):
			return false, ErrorWaitingForWorkerTimeout
		}
	}

	dms.mutex.Lock()
	dms.voiceChannelID = voiceChannelID
	dms.mutex.Unlock()

	return isWorkerActive, nil
}

func (dms *DiscordMusicSession) ClearPlaylist() {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()
	dms.currentPlaylist = nil
}

func (dms *DiscordMusicSession) GetCurrentPlaylist() entities.Playlist {
	dms.mutex.RLock()
	defer dms.mutex.RUnlock()
	return dms.currentPlaylist
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

func (dms *DiscordMusicSession) Replay() error {
	err := dms.sendCommand(dms.chanReplayCommand)

	if err == nil {
		return nil
	}

	if errors.Is(err, ErrorWorkerNotActive) {
		dms.mutex.RLock()
		lastCompletedMedia := dms.lastCompletedMedia
		dms.mutex.RUnlock()

		if lastCompletedMedia == nil {
			return ErrorNoMediaFound
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

func (dms *DiscordMusicSession) Jump(jumpTo time.Duration) error {
	if jumpTo < 0 {
		return ErrorInvalidArgument
	}

	currentMedia := dms.GetCurrentlyPlayingMedia()

	if currentMedia == nil {
		return ErrorNoMediaFound
	}

	if !currentMedia.CanJumpToTimeStamp() {
		return ErrorMediaUnsupportedFeature
	}

	duration := currentMedia.Duration()

	if duration != nil {
		if *duration < jumpTo {
			return ErrorInvalidArgument
		}
	}

	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	if !dms.workerActive {
		return ErrorWorkerNotActive
	}

	if len(dms.chanJumpCommand) > 0 {
		return ErrorCommandAlreadySent
	}

	dms.chanJumpCommand <- jumpTo
	return nil
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

func (dms *DiscordMusicSession) AddNextMediaCallback(cb NextMediaCallback) {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	dms.nextMediaCallbacks = append(dms.nextMediaCallbacks, cb)
}

func (dms *DiscordMusicSession) AddErrorCallback(cb ErrorCallback) {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	dms.errorCallbacks = append(dms.errorCallbacks, cb)
}

func (dms *DiscordMusicSession) GetVoiceChannelID() string {
	dms.mutex.RLock()
	defer dms.mutex.RUnlock()
	return dms.voiceChannelID
}

func (dms *DiscordMusicSession) GetGuildID() string {
	return dms.guildID
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
