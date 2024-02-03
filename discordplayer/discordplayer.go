package discordplayer

import (
	"musicbot/entities"
	"sync"
)

type DiscordMusicSession struct {
	mutex sync.RWMutex

	voiceChannelID  string
	voiceConnection DiscordVoiceConnection

	dca DiscordAudio

	mediaQueue []entities.Media

	chanLeaveCommand chan bool
}

func NewDiscordMusicSession(
	dca DiscordAudio,
	discord DiscordSession,
	guildId string,
	voiceChannelID string,
) (*DiscordMusicSession, error) {
	voiceConnection, err := discord.ChannelVoiceJoin(guildId, voiceChannelID, false, false)

	if err != nil {
		return nil, err
	}

	dms := &DiscordMusicSession{
		voiceChannelID:   voiceChannelID,
		voiceConnection:  voiceConnection,
		dca:              dca,
		mediaQueue:       make([]entities.Media, 0),
		chanLeaveCommand: make(chan bool, 1),
	}

	go dms.voiceWorker()

	return dms, nil
}

func (dms *DiscordMusicSession) EnqueueMedia(media entities.Media) {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	dms.mediaQueue = append(dms.mediaQueue, media)
}

func (dms *DiscordMusicSession) StartPlaylist(playlist entities.Playlist) {
}

func (dms *DiscordMusicSession) ClearPlaylist() {
}

func (dms *DiscordMusicSession) Repeat() {
}

func (dms *DiscordMusicSession) Skip() {
}

func (dms *DiscordMusicSession) Leave() {
	dms.mutex.Lock()
	defer dms.mutex.Unlock()

	if len(dms.chanLeaveCommand) == 0 {
		dms.chanLeaveCommand <- true
	}
}
