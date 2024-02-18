package discordinterface

import "github.com/bwmarrin/discordgo"

type DiscordGuild interface {
	GetVoiceStates() []DiscordVoiceState
}

type DefaultDiscordGuild struct {
	guild *discordgo.Guild
}

func (ddg *DefaultDiscordGuild) GetVoiceStates() []DiscordVoiceState {
	states := make([]DiscordVoiceState, len(ddg.guild.VoiceStates))
	for index, vs := range ddg.guild.VoiceStates {
		states[index] = NewDiscordVoiceState(vs)
	}
	return states
}

func NewDiscordGuild(guild *discordgo.Guild) DiscordGuild {
	return &DefaultDiscordGuild{
		guild: guild,
	}
}
