## Discord music streaming bot

### Generating mocks for tests
```shell
mockgen -source=discordplayer/interfaces/dcainterface.go -destination discordplayer/mocks/dcainterface_mock.go \
mockgen -source=discordplayer/interfaces/discordvoiceinterface.go -destination discordplayer/mocks/discordvoiceinterface_mock.go \
mockgen -source=discordplayer/interfaces/discordsessioninterface.go -destination discordplayer/mocks/discordsessioninterface_mock.go \
mockgen -source=discordplayer/interfaces/dcastreamingsessioninterface.go -destination discordplayer/mocks/dcastreamingsessioninterface_mock.go
```

### Todo list
- Skip command
- Repeat command
- Playlist
- Pause/Resume command
- Jump command
- Search API
- Support for media from other sources than YT (twitter?)
- Reload on FileURL expiration
- Channel interface when media changes
- Logger & error handling in discordplayer/voiceworker

#### Done
- Current playback duration API
- Reload on discord voice error
- Reconnects to voice on error
- Proper queue implementation with limit
	- Queue API to get current media in queue & size
	- Current media API
	- Clear media queue API