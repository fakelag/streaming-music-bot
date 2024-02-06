package discordplayer

import (
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

func (dms *DiscordMusicSession) voiceWorker() {
workerloop:
	for {
		mediaFile := dms.consumeNextMediaFile()

		if mediaFile != nil {
			err, keepPlaying := dms.playMediaFile(mediaFile, time.Duration(0))

			if err != nil {
				fmt.Printf("Error occurred while playing: %s\n", err.Error())
			}

			if !keepPlaying {
				break workerloop
			}
		}

		select {
		case <-dms.chanLeaveCommand:
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

	dms.disconnectAndExitWorker()
}

func (dms *DiscordMusicSession) playMediaFile(
	mediaFile entities.Media,
	startPlaybackAt time.Duration,
) (err error, keepPlaying bool) {
	err = dms.checkDiscordVoiceConnection()

	if err != nil {
		return err, true
	}

	fmt.Printf("Playing: %s\n", mediaFile.FileURL())

	_ = dms.voiceConnection.Speaking(true)

	session, err := dms.playUrlInDiscord(mediaFile.FileURL(), startPlaybackAt)

	if err != nil {
		return err, true
	}

	dms.setCurrentlyPlayingMediaAndSession(mediaFile, session)
	defer dms.setCurrentlyPlayingMediaAndSession(nil, nil)
	defer dms.setLastCompletedMedia(mediaFile)

	select {
	case err := <-session.done:
		fmt.Printf("done: %+v\n", err)

		if session.encodingSession != nil {
			session.encodingSession.Cleanup()
		}

		_ = dms.voiceConnection.Speaking(false)

		if err == nil || err == io.EOF {
			return nil, true
		}

		if !strings.Contains(err.Error(), "Voice connection closed") {
			return err, true
		}

		mediaFileDuration := mediaFile.Duration()

		if mediaFileDuration != nil {
			mediaDurationLeft := *mediaFile.Duration() - session.streamingSession.PlaybackPosition()

			if mediaDurationLeft.Seconds() < 2 {
				// No more content to play, done
				return err, true
			}
		}

		reloadPlaybackStartAt := time.Duration(0)

		if mediaFile.CanReloadFromTimeStamp() {
			reloadPlaybackStartAt = session.streamingSession.PlaybackPosition()
		}

		// Retry after voice connection drop
		return dms.playMediaFile(mediaFile, reloadPlaybackStartAt)
	case <-dms.chanLeaveCommand:
		fmt.Printf("Bot asked to leave while playing\n")

		if session.encodingSession != nil {
			session.encodingSession.Cleanup()
		}

		_ = dms.voiceConnection.Speaking(false)

		return nil, false
	case <-dms.chanSkipCommand:
		_ = dms.voiceConnection.Speaking(false)
		return nil, true
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

func (dms *DiscordMusicSession) consumeNextMediaFile() entities.Media {
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

	dms.workerActive = false
	dms.currentlyPlayingMedia = nil
	dms.mediaQueue = make([]entities.Media, 0)
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
