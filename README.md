## Discord music streaming bot

### Generating mocks for tests
```shell
mockgen -source=discordplayer/interfaces/dcainterface.go -destination discordplayer/mocks/dcainterface_mock.go \
mockgen -source=discordplayer/interfaces/discordvoiceinterface.go -destination discordplayer/mocks/discordvoiceinterface_mock.go \
mockgen -source=discordplayer/interfaces/discordsessioninterface.go -destination discordplayer/mocks/discordsessioninterface_mock.go
```

### Todo list
- Proper queue implementation with limit
	- Queue API to get current media in queue & size
	- Current media API
	- Clear media queue API
- Current playback duration API
- Skip command
- Repeat command
- Playlist
- Reconnects to voice on error
- Pause/Resume command
- Jump command
- Search API
- Support for media from other sources than YT (twitter?)
- Reloads on FileURL expiration
- Channel interface when media changes
