## Discord music streaming bot

### Generating mocks for tests
```shell
mockgen -source=discordplayer/dcainterface.go -destination mock_discordplayer/dcainterface_mock.go \
mockgen -source=discordplayer/discordvoiceinterface.go -destination mock_discordplayer/discordvoiceinterface_mock.go \
mockgen -source=discordplayer/discordsessioninterface.go -destination mock_discordplayer/discordsessioninterface_mock.go
```