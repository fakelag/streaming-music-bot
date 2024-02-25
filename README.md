## Streaming music bot library for Discord
![Coverage](https://img.shields.io/badge/Coverage-92.4%25-brightgreen)

This is a library for creating discord music bots. It can be used to bootstrap a new bot or added to an existing golang discord project. [discordgo](https://github.com/bwmarrin/discordgo) is used to interface with discord. There are built in features for most use-cases such as playing from youtube (using [ytdlp](https://github.com/yt-dlp/yt-dlp)), support for livestreams & playlists, extensive API for various commands like skip/replay/pause/jump/leave, etc. Goal of this project is to provide most common music bot functionality with a simple API that can be dropped into any discordgo project in a plug-and-play fashion.

## Dependencies
### Following commands are required in $PATH
- [ffmpeg](https://ffmpeg.org/)
- [ytdlp](https://github.com/yt-dlp/yt-dlp)

## Tests
### Generating mocks for tests
```shell
mockgen -source=discordplayer/interfaces/dcainterface.go -destination discordplayer/mocks/dcainterface_mock.go \
mockgen -source=discordplayer/interfaces/discordvoiceinterface.go -destination discordplayer/mocks/discordvoiceinterface_mock.go \
mockgen -source=discordplayer/interfaces/discordsessioninterface.go -destination discordplayer/mocks/discordsessioninterface_mock.go \
mockgen -source=discordplayer/interfaces/dcastreamingsessioninterface.go -destination discordplayer/mocks/dcastreamingsessioninterface_mock.go \
mockgen -source=discordplayer/interfaces/discordguildinterface.go -destination discordplayer/mocks/discordguildinterface_mock.go \
mockgen -source=discordplayer/interfaces/discordvoicestateinterface.go -destination discordplayer/mocks/discordvoicestateinterface_mock.go \
mockgen -source=discordplayer/interfaces/discorduserinterface.go -destination discordplayer/mocks/discorduserinterface_mock.go
```
### Running tests
Test suite can be run with ginkgo
```shell
ginkgo -r -p
```

## Usage
```golang
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
	"github.com/fakelag/streaming-music-bot/discordplayer"
	"github.com/fakelag/streaming-music-bot/youtubeapi"
)

const GuildID = "YOUR_GUILD_ID"
const VoiceChannelID = "YOUR_VOICE_CHANNEL_ID"
const TextChannelID = "YOUR_TEXT_CHANNEL_ID"
const SearchTerm = "YOUR_YT_SEARCH_TERM"
const DiscordToken = "YOUR_DISCORD_TOKEN"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	discord, err := discordgo.New("Bot " + DiscordToken)

	if err != nil {
		panic(err)
	}

	discord.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAllWithoutPrivileged | discordgo.IntentMessageContent)
	err = discord.Open()

	if err != nil {
		panic(err)
	}

	defer discord.Close()

	session, err := discordplayer.NewDiscordMusicSession(ctx, discord, &discordplayer.DiscordMusicSessionOptions{
		GuildID:           GuildID,
		VoiceChannelID:    VoiceChannelID,
		MediaQueueMaxSize: MaxQueueSize,
	})

	if err != nil {
		panic(err)
	}

	youtubeApi = youtubeapi.NewYoutubeAPI()
	media, err := youtubeApi.GetYoutubeMedia(SearchTerm)

	if err != nil {
		panic(err)
	}

	err = session.EnqueueMedia(media)

	if err != nil {
		if errors.Is(err, discordplayer.ErrorMediaQueueFull) {
			discord.ChannelMessageSend(TextChannelID, "Queue is full")
		} else {
			discord.ChannelMessageSend(TextChannelID, fmt.Sprintf("Failed to enqueue media: %s", err.Error()))
		}
	} else {
		discord.ChannelMessageSend(TextChannelID, fmt.Sprintf("Added %s to the queue", media.Title()))
	}

	fmt.Println("Running...")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	fmt.Println("Exiting...")
	return
}
```

// test

#### Features
- Media change callbacks
- Context based timeout
- Reload on FileURL expiration
- Playlists
- Jump command
- Pause/Resume command
- Repeat command
- Skip command
- Current playback duration API
- Reload on discord voice error
- Reconnects to voice on error
- Media queue with maximum size
	- Queue API to get current media in queue & size
	- Currently playing media API
	- Clear queue API