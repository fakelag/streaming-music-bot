package youtube

import (
	"musicbot/entities"
	"time"
)

type YoutubeMedia struct {
	ID              string
	Title           string
	StreamURL       string
	StreamExpiresAt time.Time
}

func (ytm *YoutubeMedia) FileURL() string {
	return ytm.StreamURL
}

// Verify implements entities.Media
var _ entities.Media = (*YoutubeMedia)(nil)
