package youtube

import "time"

type YoutubeMedia struct {
	ID              string
	Title           string
	StreamURL       string
	StreamExpiresAt time.Time
}

// Verify implements entities.Media
// var _ entities.Media = YoutubeMedia{}
// var _ entities.Media = (*YoutubeMedia)(nil)
