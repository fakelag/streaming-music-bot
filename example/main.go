package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	discordplayer "github.com/fakelag/streaming-music-bot/discordplayer"
	"github.com/fakelag/streaming-music-bot/youtubeapi"

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

	ytSearchTerm, ok := os.LookupEnv("YT_SEARCH_TERM")

	if !ok {
		panic("missing YT_SEARCH_TERM")
	}

	err = discord.Open()

	if err != nil {
		panic(err)
	}

	defer discord.Close()

	yt := youtubeapi.NewYoutubeAPI()
	media, err := yt.GetYoutubeMedia(ytSearchTerm)

	if err != nil {
		panic(err)
	}

	time.Sleep(2 * time.Second)
	fmt.Println("Running...")

	guild := discord.State.Guilds[0]

	channelID := ""
	for _, c := range guild.Channels {
		if c.Name == "General" {
			channelID = c.ID
		}
	}

	dms, err := discordplayer.NewDiscordMusicSession(context.TODO(), discord, &discordplayer.DiscordMusicSessionOptions{
		GuildID:           guild.ID,
		VoiceChannelID:    channelID,
		MediaQueueMaxSize: 10,
	})

	if err != nil {
		panic(err)
	}

	dms.EnqueueMedia(media)

	waitSigTerm()

	dms.Leave()

	fmt.Println("Exiting...")
	return
}
