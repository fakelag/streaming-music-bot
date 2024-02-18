package discordinterface

import "github.com/bwmarrin/discordgo"

type DiscordUser interface {
	Bot() bool
}

type DefaultDiscordUser struct {
	user *discordgo.User
}

func (dvs *DefaultDiscordUser) Bot() bool {
	return dvs.user.Bot
}

func NewDiscordUser(user *discordgo.User) DiscordUser {
	return &DefaultDiscordUser{
		user: user,
	}
}
