package discordplayer

import "github.com/bwmarrin/discordgo"

type DiscordSession interface {
	ChannelVoiceJoin(gID string, cID string, mute bool, deaf bool) (voice DiscordVoiceConnection, err error)
}

type DefaultDiscordSession struct {
	session *discordgo.Session
}

func (dds *DefaultDiscordSession) ChannelVoiceJoin(gID string, cID string, mute bool, deaf bool) (voice DiscordVoiceConnection, err error) {
	voiceConn, err := dds.session.ChannelVoiceJoin(gID, cID, mute, deaf)

	if err != nil {
		return nil, err
	}

	return &DefaultDiscordVoiceConnection{
		voiceConn: voiceConn,
	}, nil
}

func NewDiscordSession(discord *discordgo.Session) DiscordSession {
	return &DefaultDiscordSession{
		session: discord,
	}
}
