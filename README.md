## Discord music streaming bot

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

### Todo list
- Search API
- Support for media from other sources than YT (twitter?)
- Logger & error handling in discordplayer/voiceworker

#### Done
- Media change callbacks
- Context based timeout
- Reload on FileURL expiration
- Playlist
- Jump command
- Pause/Resume command
- Repeat command
- Skip command
- Current playback duration API
- Reload on discord voice error
- Reconnects to voice on error
- Proper queue implementation with limit
	- Queue API to get current media in queue & size
	- Current media API
	- Clear media queue API