package discordplayer

import (
	"musicbot/entities"
	"sync"

	"github.com/bwmarrin/discordgo"
)

type DiscordMusicSession struct {
	mutex sync.RWMutex

	voiceChannelID  string
	voiceConnection *discordgo.VoiceConnection

	mediaQueue []entities.Media
}

func NewDiscordMusicSession(discord *discordgo.Session, guildId string, voiceChannelID string) (*DiscordMusicSession, error) {
	voiceConnection, err := discord.ChannelVoiceJoin(guildId, voiceChannelID, false, false)

	if err != nil {
		return nil, err
	}

	dms := &DiscordMusicSession{
		voiceChannelID:  voiceChannelID,
		voiceConnection: voiceConnection,
		mediaQueue:      make([]entities.Media, 0),
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
}