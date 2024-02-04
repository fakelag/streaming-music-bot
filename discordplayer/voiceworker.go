package discordplayer

import (
	"fmt"
	"io"
	"musicbot/entities"
	"time"

	"github.com/fakelag/dca"
)

type DcaMediaSession struct {
	encodingSession  *dca.EncodeSession
	streamingSession *dca.StreamingSession
	done             chan error
}

func (dms *DiscordMusicSession) voiceWorker() {
	defer dms.disconnectAndExitWorker()

workerloop:
	for {
		mediaFile := dms.consumeNextMediaFile()

		if mediaFile != nil {
			err, keepPlaying := dms.playMediaFile(mediaFile)

			if err != nil {
				panic(err)
			}

			if !keepPlaying {
				break workerloop
			}
		}

		select {
		case <-dms.chanLeaveCommand:
			break workerloop
		default:
			break
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func (dms *DiscordMusicSession) playMediaFile(mediaFile entities.Media) (err error, keepPlaying bool) {
	err = dms.checkDiscordVoiceConnection()

	if err != nil {
		return err, true
	}

	fmt.Printf("Playing: %s\n", mediaFile.FileURL())

	_ = dms.voiceConnection.Speaking(true)

	session, err := dms.playUrlInDiscord(mediaFile.FileURL(), time.Duration(0))

	if err != nil {
		return err, true
	}

	dms.setCurrentlyPlayingMedia(mediaFile)
	defer dms.setCurrentlyPlayingMedia(nil) // TODO: Think about this again when implementing reloads

	select {
	case err := <-session.done:
		fmt.Printf("done: %+v\n", err)

		if session.encodingSession != nil {
			session.encodingSession.Cleanup()
		}

		_ = dms.voiceConnection.Speaking(false)

		if err != nil && err != io.EOF {
			return err, true
		}

		return nil, true
	case <-dms.chanLeaveCommand:
		fmt.Printf("Bot asked to leave while playing\n")

		if session.encodingSession != nil {
			session.encodingSession.Cleanup()
		}

		_ = dms.voiceConnection.Speaking(false)

		return nil, false
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

func (dms *DiscordMusicSession) setCurrentlyPlayingMedia(media entities.Media) {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()
	dms.currentlyPlayingMedia = media
}
