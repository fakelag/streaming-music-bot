package discordinterface

import (
	"time"

	"github.com/fakelag/dca"
)

type DcaStreamingSession interface {
	SetPaused(paused bool)
	PlaybackPosition() time.Duration
	Finished() (bool, error)
	Paused() bool
}

type DefaultDcaStreamingSession struct {
	streamingSession *dca.StreamingSession
}

func (ddss *DefaultDcaStreamingSession) SetPaused(paused bool) {
	ddss.streamingSession.SetPaused(paused)
}

func (ddss *DefaultDcaStreamingSession) PlaybackPosition() time.Duration {
	return ddss.streamingSession.PlaybackPosition()
}

func (ddss *DefaultDcaStreamingSession) Finished() (bool, error) {
	return ddss.streamingSession.Finished()
}

func (ddss *DefaultDcaStreamingSession) Paused() bool {
	return ddss.streamingSession.Paused()
}

func NewDcaStreamingSession(streamingSession *dca.StreamingSession) DcaStreamingSession {
	dcaStreamingSession := &DefaultDcaStreamingSession{
		streamingSession: streamingSession,
	}
	return dcaStreamingSession
}
