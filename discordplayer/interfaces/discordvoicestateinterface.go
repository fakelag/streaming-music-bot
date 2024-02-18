package discordinterface

import "github.com/bwmarrin/discordgo"

type DiscordVoiceState interface {
	GetChannelID() string
	GetUserID() string
}

type DefaultDiscordVoiceState struct {
	voiceState *discordgo.VoiceState
}

func (dvs *DefaultDiscordVoiceState) GetChannelID() string {
	return dvs.voiceState.ChannelID
}

func (dvs *DefaultDiscordVoiceState) GetUserID() string {
	return dvs.voiceState.UserID
}

func NewDiscordVoiceState(vs *discordgo.VoiceState) DiscordVoiceState {
	return &DefaultDiscordVoiceState{
		voiceState: vs,
	}
}
