package discordplayer

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	discordinterface "github.com/fakelag/streaming-music-bot/discordplayer/interfaces"
	"github.com/fakelag/streaming-music-bot/entities"

	"github.com/fakelag/dca"
	// . "github.com/onsi/ginkgo/v2"
)

type DcaMediaSession struct {
	encodingSession  *dca.EncodeSession
	streamingSession discordinterface.DcaStreamingSession
	done             chan error
}

func (dms *DiscordMusicSession) voiceWorker(done context.CancelFunc) {
	defer done()
	defer dms.disconnectAndExitWorker()

	ctx, cancel := dms.voiceWorkerContext()
	defer cancel()

workerloop:
	for {
		mediaFile := dms.consumeNextMediaFromQueue()

		if mediaFile == nil {
			mediaFile = dms.consumeNextMediaFromPlaylist()
		}

		if mediaFile != nil {
			keepPlayingCurrentMedia := true
			keepPlayingCurrentMediaFrom := time.Duration(0)
			isReload := false

			for keepPlayingCurrentMedia {
				var err error
				var exitWorker bool

				dms.invokeNextMediaCallbacks(mediaFile, isReload)
				err, exitWorker, keepPlayingCurrentMedia, keepPlayingCurrentMediaFrom = dms.playMediaFile(
					ctx,
					mediaFile,
					keepPlayingCurrentMediaFrom,
				)

				isReload = true

				if err != nil {
					dms.invokeErrorCallbacks(mediaFile, err)
				}

				if exitWorker {
					break workerloop
				}
			}

		}

		select {
		case <-dms.chanLeaveCommand:
			break workerloop
		case <-ctx.Done():
			break workerloop
		case <-dms.chanReplayCommand:
			dms.mutex.RLock()
			repeatMedia := dms.lastCompletedMedia
			dms.mutex.RUnlock()

			if repeatMedia != nil {
				// TODO repeat from from start of the queue
				dms.EnqueueMedia(repeatMedia)
			}
		default:
			break
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func (dms *DiscordMusicSession) playMediaFile(
	ctx context.Context,
	mediaFile entities.Media,
	startPlaybackAt time.Duration,
) (
	err error,
	exitWorker bool,
	keepPlayingCurrentMedia bool,
	keepPlayingCurrentMediaFrom time.Duration,
) {
	keepPlayingCurrentMedia = false
	keepPlayingCurrentMediaFrom = time.Duration(0)

	playMediaCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	err = dms.checkDiscordVoiceConnection()

	if err != nil {
		return
	}

	err = mediaFile.EnsureLoaded()

	if err != nil {
		return
	}

	_ = dms.voiceConnection.Speaking(true)

	session, err := dms.playUrlInDiscord(mediaFile.FileURL(), startPlaybackAt)

	if err != nil {
		return
	}

	dms.setCurrentlyPlayingMediaAndSession(mediaFile, session)
	defer dms.setCurrentlyPlayingMediaAndSession(nil, nil)
	defer dms.setLastCompletedMedia(mediaFile)

	fileUrlExpiresAt := mediaFile.FileURLExpiresAt()
	reloadChan := make(chan bool, 1)

	if fileUrlExpiresAt != nil {
		go dms.checkForMediaFileExpiration(playMediaCtx, fileUrlExpiresAt, reloadChan)
	}

	select {
	case err = <-session.done:
		dms.cleanupEncodingAndVoiceSession(session.encodingSession, dms.voiceConnection)

		if err == nil || err == io.EOF {
			err = nil
			return
		}

		if !strings.Contains(err.Error(), "Voice connection closed") {
			return
		}

		mediaFileDuration := mediaFile.Duration()

		if mediaFileDuration != nil {
			mediaDurationLeft := *mediaFileDuration - session.streamingSession.PlaybackPosition()

			if mediaDurationLeft.Seconds() < 2 {
				// No more content to play, done
				return
			}
		}

		keepPlayingCurrentMedia = true

		if mediaFile.CanJumpToTimeStamp() {
			keepPlayingCurrentMediaFrom = session.streamingSession.PlaybackPosition()
		}

		return
	case <-dms.chanLeaveCommand:
		dms.cleanupEncodingAndVoiceSession(session.encodingSession, dms.voiceConnection)

		exitWorker = true
		return
	case jumpTo := <-dms.chanJumpCommand:
		dms.cleanupEncodingAndVoiceSession(session.encodingSession, dms.voiceConnection)

		keepPlayingCurrentMedia = true
		keepPlayingCurrentMediaFrom = jumpTo
		return
	case <-dms.chanSkipCommand:
		dms.cleanupEncodingAndVoiceSession(session.encodingSession, dms.voiceConnection)
		return
	case <-reloadChan:
		dms.cleanupEncodingAndVoiceSession(session.encodingSession, dms.voiceConnection)

		keepPlayingCurrentMedia = true

		if mediaFile.CanJumpToTimeStamp() {
			keepPlayingCurrentMediaFrom = session.streamingSession.PlaybackPosition()
		}

		return
	case <-playMediaCtx.Done():
		dms.cleanupEncodingAndVoiceSession(session.encodingSession, dms.voiceConnection)

		exitWorker = true
		return
	}
}

func (dms *DiscordMusicSession) playUrlInDiscord(url string, startPlaybackAt time.Duration) (*DcaMediaSession, error) {
	options := dca.StdEncodeOptions
	options.RawOutput = true
	options.Bitrate = 96
	options.Application = "lowdelay"
	options.StartTime = int(startPlaybackAt.Seconds())

	encodingSession, err := dms.dca.EncodeFile(url, options)

	if err != nil {
		return nil, err
	}

	done := make(chan error)

	time.Sleep(250 * time.Millisecond)
	streamingSession := dms.dca.NewStream(encodingSession, dms.voiceConnection, done)

	return &DcaMediaSession{
		encodingSession:  encodingSession,
		streamingSession: streamingSession,
		done:             done,
	}, nil
}

func (dms *DiscordMusicSession) consumeNextMediaFromQueue() entities.Media {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	if len(dms.mediaQueue) == 0 {
		return nil
	}

	var nextMediaFile entities.Media

	// Queue is resized when consuming media
	nextMediaFile, dms.mediaQueue = dms.mediaQueue[0], dms.mediaQueue[1:]

	return nextMediaFile
}

func (dms *DiscordMusicSession) consumeNextMediaFromPlaylist() entities.Media {
	dms.mutex.RLock()

	if dms.currentPlaylist == nil {
		dms.mutex.RUnlock()
		return nil
	}

	media, err := dms.currentPlaylist.ConsumeNextMedia()

	dms.mutex.RUnlock()

	if err != nil {
		if errors.Is(err, entities.ErrorPlaylistEmpty) {
			dms.ClearPlaylist()
			return nil
		}
		// TODO: log error
		return nil
	}

	return media
}

func (dms *DiscordMusicSession) checkDiscordVoiceConnection() error {
	if dms.voiceConnection != nil && dms.voiceConnection.IsReady() {
		return nil
	}

	dms.mutex.RLock()
	voiceChannelID := dms.voiceChannelID
	dms.mutex.RUnlock()

	newVoiceConnection, err := dms.discordSession.ChannelVoiceJoin(dms.guildID, voiceChannelID, false, false)

	if err != nil {
		return err
	}

	dms.voiceConnection = newVoiceConnection
	return nil
}

func (dms *DiscordMusicSession) voiceWorkerContext() (ctx context.Context, cancel context.CancelFunc) {
	ctx, cancel = context.WithCancel(dms.workerCtx)

	checkChannelEmpty := dms.leaveAfterChannelEmptyTime != time.Duration(0)
	checkQueueEmpty := dms.leaveAfterEmptyQueueTime != time.Duration(0)

	if !checkChannelEmpty && !checkQueueEmpty {
		return
	}

	dms.mutex.RLock()
	// TODO - Get voice channel from dms.voiceConnection instead during the loop
	// to check current channel in the case bot was switched
	voiceChannelID := dms.voiceChannelID
	dms.mutex.RUnlock()

	go func() {
		channelNotEmptyAt := time.Now()
		queueNotEmptyAt := time.Now()

		for {
			if checkQueueEmpty {
				currentMedia := dms.GetCurrentlyPlayingMedia()

				if currentMedia != nil {
					queueNotEmptyAt = time.Now()
				} else if time.Since(queueNotEmptyAt) >= dms.leaveAfterEmptyQueueTime {
					cancel()
					return
				}
			}

			if checkChannelEmpty {
				hasNonBotMembers, err := dms.hasNonBotMembersInVoiceChannel(voiceChannelID)

				if err != nil || hasNonBotMembers {
					channelNotEmptyAt = time.Now()
				} else if time.Since(channelNotEmptyAt) >= dms.leaveAfterChannelEmptyTime {
					cancel()
					return
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(dms.leaveAfterCheckInterval):
				break
			}
		}
	}()

	return
}

func (dms *DiscordMusicSession) hasNonBotMembersInVoiceChannel(voiceChannelID string) (bool, error) {
	guild, err := dms.discordSession.Guild(dms.guildID)

	if err != nil {
		return false, err
	}

	if guild == nil {
		return false, errors.New("guild not found")
	}

	voiceStates := guild.GetVoiceStates()

	hasNonBotMembersInVC := false

	for _, vs := range voiceStates {
		if voiceChannelID != vs.GetChannelID() {
			continue
		}

		user, err := dms.discordSession.User(vs.GetUserID())

		if err != nil {
			return false, err
		}

		if !user.Bot() {
			hasNonBotMembersInVC = true
			break
		}
	}

	return hasNonBotMembersInVC, nil
}

func (dms *DiscordMusicSession) disconnectAndExitWorker() {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	close(dms.chanJumpCommand)
	close(dms.chanLeaveCommand)
	close(dms.chanReplayCommand)
	close(dms.chanSkipCommand)

	dms.workerActive = false
	dms.currentlyPlayingMedia = nil
	dms.mediaQueue = make([]entities.Media, 0)
	dms.currentPlaylist = nil

	if dms.voiceConnection != nil {
		dms.voiceConnection.Disconnect()
		dms.voiceConnection = nil
	}
}

func (dms *DiscordMusicSession) setCurrentlyPlayingMediaAndSession(media entities.Media, session *DcaMediaSession) {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()
	dms.currentlyPlayingMedia = media
	dms.currentMediaSession = session
}

func (dms *DiscordMusicSession) setLastCompletedMedia(media entities.Media) {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()
	dms.lastCompletedMedia = media
}

func (dms *DiscordMusicSession) cleanupEncodingAndVoiceSession(
	encodingSession *dca.EncodeSession,
	voiceConnection discordinterface.DiscordVoiceConnection,
) {
	if encodingSession != nil {
		encodingSession.Cleanup()
	}

	_ = voiceConnection.Speaking(false)
}

func (dms *DiscordMusicSession) checkForMediaFileExpiration(
	ctx context.Context,
	fileUrlExpiresAt *time.Time,
	reloadChan chan bool,
) {
	for {
		if time.Since(*fileUrlExpiresAt) > -10*time.Second {
			close(reloadChan)
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
			break
		}
	}
}

func (dms *DiscordMusicSession) invokeNextMediaCallbacks(mediaFile entities.Media, isReload bool) {
	dms.mutex.RLock()
	defer dms.mutex.RUnlock()

	for _, cb := range dms.nextMediaCallbacks {
		go cb(dms, mediaFile, isReload)
	}
}

func (dms *DiscordMusicSession) invokeErrorCallbacks(mediaFile entities.Media, err error) {
	dms.mutex.RLock()
	defer dms.mutex.RUnlock()

	for _, cb := range dms.errorCallbacks {
		go cb(dms, mediaFile, err)
	}
}
