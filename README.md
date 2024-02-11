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