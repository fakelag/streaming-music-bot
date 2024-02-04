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
	if dms.isWorkerActive() {
		return
	}

	dms.setWorkerActive(true)
	defer dms.setWorkerActive(false)

	voiceConnection, err := dms.discordSession.ChannelVoiceJoin(dms.guildID, dms.voiceChannelID, false, false)

	if err != nil {
		panic(err)
	}

	dms.voiceConnection = voiceConnection

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
			fmt.Printf("Bot asked to leave while idle\n")
			break workerloop
		default:
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	fmt.Printf("Exiting voice channel %s\n", dms.voiceChannelID)
	dms.voiceConnection.Disconnect()
}

func (dms *DiscordMusicSession) playMediaFile(mediaFile entities.Media) (err error, keepPlaying bool) {
	// TODO check voice connection OK

	fmt.Printf("Playing: %s - %t\n", mediaFile.FileURL(), dms.voiceConnection.IsReady())

	_ = dms.voiceConnection.Speaking(true)

	session, err := dms.playUrlInDiscord(mediaFile.FileURL(), time.Duration(0))

	if err != nil {
		return err, true
	}

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

func (dms *DiscordMusicSession) setWorkerActive(active bool) {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()
	dms.workerActive = active
}

func (dms *DiscordMusicSession) isWorkerActive() bool {
	dms.mutex.RLock()
	defer dms.mutex.RUnlock()
	return dms.workerActive
}
