package youtubeapi

import (
	"math/rand"
	"sync"
	"time"

	"github.com/fakelag/streaming-music-bot/entities"
)

type YoutubePlaylist struct {
	sync.RWMutex

	ID            string
	PlaylistTitle string
	PlaylistLink  string

	removeMediaOnConsume bool
	consumeOrder         entities.PlaylistConsumeOrder
	mediaList            []*YoutubeMedia
	nextMediaIndex       int

	// mutex needs to be write-locked for rng
	rng *rand.Rand
}

func (ypl *YoutubePlaylist) Title() string {
	return ypl.PlaylistTitle
}

func (ypl *YoutubePlaylist) Link() string {
	return ypl.PlaylistLink
}

func (ypl *YoutubePlaylist) ConsumeNextMedia() (entities.Media, error) {
	mediaIndex := -1

	ypl.Lock()
	defer ypl.Unlock()

	if len(ypl.mediaList) == 0 {
		return nil, entities.ErrorPlaylistEmpty
	}

	switch ypl.consumeOrder {
	case entities.ConsumeOrderFromStart:
		mediaIndex = ypl.nextMediaIndex % len(ypl.mediaList)
		ypl.nextMediaIndex += 1
	case entities.ConsumeOrderShuffle:
		mediaIndex = ypl.rng.Intn(len(ypl.mediaList))
	default:
		break
	}

	selectedMediaFile := ypl.mediaList[mediaIndex]

	if !ypl.removeMediaOnConsume {
		return selectedMediaFile, nil
	}

	newMediaList := make([]*YoutubeMedia, len(ypl.mediaList)-1)
	newMediaIndex := 0

	for index, media := range ypl.mediaList {
		if index == mediaIndex {
			continue
		}

		newMediaList[newMediaIndex] = media
		newMediaIndex += 1
	}

	ypl.mediaList = newMediaList

	return selectedMediaFile, nil
}

func (ypl *YoutubePlaylist) SetConsumeOrder(order entities.PlaylistConsumeOrder) error {
	if order != entities.ConsumeOrderFromStart && order != entities.ConsumeOrderShuffle {
		return entities.ErrorConsumeOrderNotSupported
	}

	ypl.Lock()
	defer ypl.Unlock()
	ypl.consumeOrder = order
	return nil
}

func (ypl *YoutubePlaylist) SetRemoveOnConsume(removeMediaOnConsume bool) {
	ypl.Lock()
	defer ypl.Unlock()
	ypl.removeMediaOnConsume = removeMediaOnConsume
}

func (ypl *YoutubePlaylist) GetAvailableConsumeOrders() []entities.PlaylistConsumeOrder {
	return []entities.PlaylistConsumeOrder{entities.ConsumeOrderFromStart, entities.ConsumeOrderShuffle}
}

func (ypl *YoutubePlaylist) GetMediaCount() int {
	ypl.RLock()
	defer ypl.RUnlock()
	return len(ypl.mediaList)
}

func (ypl *YoutubePlaylist) GetRemoveOnConsume() bool {
	ypl.RLock()
	defer ypl.RUnlock()
	return ypl.removeMediaOnConsume
}

func (ypl *YoutubePlaylist) GetConsumeOrder() entities.PlaylistConsumeOrder {
	ypl.RLock()
	defer ypl.RUnlock()
	return ypl.consumeOrder
}

func (ypl *YoutubePlaylist) GetDurationLeft() *time.Duration {
	ypl.RLock()
	defer ypl.RUnlock()

	durationLeft := time.Duration(0)
	for _, media := range ypl.mediaList {
		mediaDuration := media.Duration()

		if mediaDuration != nil {
			durationLeft += *mediaDuration
		}
	}

	return &durationLeft
}

// Verify implements entities.Playlist
var _ entities.Playlist = (*YoutubePlaylist)(nil)
