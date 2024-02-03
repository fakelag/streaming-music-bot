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
	for {
		mediaFile := dms.nextMediaFile()

		if mediaFile != nil {
			err := dms.playMediaFile(mediaFile)

			if err != nil {
				panic(err)
			}
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func (dms *DiscordMusicSession) playMediaFile(mediaFile entities.Media) error {
	// TODO check voice connection OK

	fmt.Printf("Playing: %s - %t\n", mediaFile.FileURL(), dms.voiceConnection.IsReady())

	_ = dms.voiceConnection.Speaking(true)

	session, err := dms.playUrlInDiscord(mediaFile.FileURL(), time.Duration(0))

	if err != nil {
		return err
	}

	select {
	case err := <-session.done:
		fmt.Printf("done: %+v\n", err)

		if session.encodingSession != nil {
			session.encodingSession.Cleanup()
		}

		_ = dms.voiceConnection.Speaking(false)

		if err != nil && err != io.EOF {
			return err
		}

		return nil
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

func (dms *DiscordMusicSession) nextMediaFile() entities.Media {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	if len(dms.mediaQueue) == 0 {
		return nil
	}

	var nextMediaFile entities.Media
	nextMediaFile, dms.mediaQueue = dms.mediaQueue[0], dms.mediaQueue[1:]
	return nextMediaFile
}
