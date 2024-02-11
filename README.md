## Streaming music bot library for Discord

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
mockgen -source=discordplayer/interfaces/dcastreamingsessioninterface.go -destination discordplayer/mocks/dcastreamingsessioninterface_mock.go
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
	"github.com/fakelag/streaming-music-bot/youtube"
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

	youtubeApi = youtube.NewYoutubeAPI()
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