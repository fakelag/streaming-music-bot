package youtube

import (
	"musicbot/entities"
	"time"
)

type YoutubeMedia struct {
	ID              string
	Title           string
	IsLiveStream    bool
	VideoDuration   time.Duration
	StreamURL       string
	StreamExpiresAt time.Time
}

func (ytm *YoutubeMedia) FileURL() string {
	return ytm.StreamURL
}

func (ytm *YoutubeMedia) CanJumpToTimeStamp() bool {
	return !ytm.IsLiveStream
}

func (ytm *YoutubeMedia) Duration() *time.Duration {
	if ytm.IsLiveStream {
		return nil
	}
	return &ytm.VideoDuration
}

// Verify implements entities.Media
var _ entities.Media = (*YoutubeMedia)(nil)
