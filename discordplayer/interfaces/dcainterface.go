package discordinterface

import (
	"github.com/fakelag/dca"
)

type DiscordAudio interface {
	NewStream(source dca.OpusReader, vc DiscordVoiceConnection, done chan error) DcaStreamingSession
	EncodeFile(path string, options *dca.EncodeOptions) (session *dca.EncodeSession, err error)
}

type DefaultDiscordAudio struct {
}

func (dda *DefaultDiscordAudio) NewStream(source dca.OpusReader, vc DiscordVoiceConnection, done chan error) DcaStreamingSession {
	dcaStreamingSession := dca.NewStream(source, vc.GetRaw(), done)
	return NewDcaStreamingSession(dcaStreamingSession)
}

func (dda *DefaultDiscordAudio) EncodeFile(path string, options *dca.EncodeOptions) (session *dca.EncodeSession, err error) {
	return dca.EncodeFile(path, options)
}

func NewDiscordAudio() DiscordAudio {
	dca := &DefaultDiscordAudio{}
	return dca
}
