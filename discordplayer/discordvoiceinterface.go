package discordplayer

import "github.com/bwmarrin/discordgo"

type DiscordVoiceConnection interface {
	Speaking(b bool) error
	IsReady() bool
	GetRaw() *discordgo.VoiceConnection
}

type DefaultDiscordVoiceConnection struct {
	voiceConn *discordgo.VoiceConnection
}

func (dvc *DefaultDiscordVoiceConnection) Speaking(b bool) error {
	return dvc.voiceConn.Speaking(b)
}

func (dvc *DefaultDiscordVoiceConnection) IsReady() bool {
	return dvc.voiceConn.Ready
}

func (dvc *DefaultDiscordVoiceConnection) GetRaw() *discordgo.VoiceConnection {
	return dvc.voiceConn
}