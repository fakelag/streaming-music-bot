## Discord music streaming bot

### Generating mocks for tests
```shell
mockgen -source=discordplayer/interfaces/dcainterface.go -destination discordplayer/mocks/dcainterface_mock.go \
mockgen -source=discordplayer/interfaces/discordvoiceinterface.go -destination discordplayer/mocks/discordvoiceinterface_mock.go \
mockgen -source=discordplayer/interfaces/discordsessioninterface.go -destination discordplayer/mocks/discordsessioninterface_mock.go
```