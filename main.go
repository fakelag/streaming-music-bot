package main

import (
	"fmt"
	discordplayer "musicbot/discordplayer"
	"os"
	"os/signal"
	"time"

	"github.com/bwmarrin/discordgo"
)

func waitSigTerm() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func main() {
	discordToken, ok := os.LookupEnv("DISCORD_TOKEN")

	if !ok {
		panic("missing DISCORD_TOKEN")
	}

	discord, err := discordgo.New("Bot " + discordToken)

	if err != nil {
		panic(err)
	}

	// discord.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAll)

	err = discord.Open()

	if err != nil {
		panic(err)
	}

	defer discord.Close()

	// yt := youtube.NewYoutubeAPI()
	// media, err := yt.GetYoutubeMedia("dQw4w9WgXcQ")

	// if err != nil {
	// 	panic(err)
	// }

	// fmt.Printf("media=%+v\n", media)

	time.Sleep(2 * time.Second)
	fmt.Println("Running...")

	guild := discord.State.Guilds[0]

	channelID := ""
	for _, c := range guild.Channels {
		if c.Name == "General" {
			channelID = c.ID
		}
	}

	dms, err := discordplayer.NewDiscordMusicSession(discord, guild.ID, channelID)

	if err != nil {
		panic(err)
	}

	// dms.EnqueueMedia(media)

	waitSigTerm()

	dms.Leave()

	fmt.Println("Exiting...")
	return
}
