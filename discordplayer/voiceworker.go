package discordplayer

import (
	"context"
	"errors"
	"fmt"
	"io"
	discordinterface "musicbot/discordplayer/interfaces"
	"musicbot/entities"
	"strings"
	"time"

	"github.com/fakelag/dca"
	// . "github.com/onsi/ginkgo/v2"
)

type DcaMediaSession struct {
	encodingSession  *dca.EncodeSession
	streamingSession discordinterface.DcaStreamingSession
	done             chan error
}

func (dms *DiscordMusicSession) voiceWorker(ctx context.Context) {
	defer dms.disconnectAndExitWorker()

workerloop:
	for {
		mediaFile := dms.consumeNextMediaFromQueue()

		if mediaFile == nil {
			mediaFile = dms.consumeNextMediaFromPlaylist()
		}

		if mediaFile != nil {
			keepPlayingCurrentMedia := true
			keepPlayingCurrentMediaFrom := time.Duration(0)

			for keepPlayingCurrentMedia {
				var err error
				var exitWorker bool

				err, exitWorker, keepPlayingCurrentMedia, keepPlayingCurrentMediaFrom = dms.playMediaFile(
					ctx,
					mediaFile,
					keepPlayingCurrentMediaFrom,
				)

				if err != nil {
					fmt.Printf("Error occurred while playing: %s\n", err.Error())
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
		case <-dms.chanRepeatCommand:
			dms.mutex.RLock()
			repeatMedia := dms.lastCompletedMedia
			dms.mutex.RUnlock()

			if repeatMedia != nil {
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

	fmt.Printf("Playing: %s\n", mediaFile.FileURL())

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
		fmt.Printf("done: %+v\n", err)

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
			mediaDurationLeft := *mediaFile.Duration() - session.streamingSession.PlaybackPosition()

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
		fmt.Printf("Bot asked to leave while playing\n")

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
		fmt.Printf("Bot context canceled. Exiting...\n")

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
	defer dms.mutex.RUnlock()

	if dms.currentPlaylist == nil {
		return nil
	}

	media, err := dms.currentPlaylist.ConsumeNextMedia()

	if err != nil {
		if errors.Is(err, entities.ErrorPlaylistEmpty) {
			// TODO: remove playlist
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

	newVoiceConnection, err := dms.discordSession.ChannelVoiceJoin(dms.guildID, dms.voiceChannelID, false, false)

	if err != nil {
		return err
	}

	dms.voiceConnection = newVoiceConnection
	return nil
}

func (dms *DiscordMusicSession) disconnectAndExitWorker() {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	close(dms.chanJumpCommand)
	close(dms.chanLeaveCommand)
	close(dms.chanRepeatCommand)
	close(dms.chanSkipCommand)

	dms.workerActive = false
	dms.currentlyPlayingMedia = nil
	dms.mediaQueue = make([]entities.Media, 0)
	dms.currentPlaylist = nil
	dms.voiceConnection.Disconnect()

	fmt.Printf("Exiting voice channel %s\n", dms.voiceChannelID)
}

func (dms *DiscordMusicSession) isWorkerActive() bool {
	dms.mutex.RLock()
	defer dms.mutex.RUnlock()
	return dms.workerActive
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
