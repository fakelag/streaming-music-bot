package discordinterface

import "github.com/bwmarrin/discordgo"

type DiscordSession interface {
	ChannelVoiceJoin(gID string, cID string, mute bool, deaf bool) (voice DiscordVoiceConnection, err error)
	Guild(gID string) (guild DiscordGuild, err error)
	User(uID string) (user DiscordUser, err error)
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

func (dds *DefaultDiscordSession) Guild(gID string) (guild DiscordGuild, err error) {
	g, err := dds.session.Guild(gID)

	if err != nil {
		return nil, err
	}

	return NewDiscordGuild(g), err
}

func (dds *DefaultDiscordSession) User(uID string) (user DiscordUser, err error) {
	u, err := dds.session.User(uID)

	if err != nil {
		return nil, err
	}

	return NewDiscordUser(u), err
}

func NewDiscordSession(discord *discordgo.Session) DiscordSession {
	return &DefaultDiscordSession{
		session: discord,
	}
}
