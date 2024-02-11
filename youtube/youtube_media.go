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
	StreamExpiresAt *time.Time
	Link            string
	ytAPI           *Youtube
}

func (ytm *YoutubeMedia) FileURL() string {
	return ytm.StreamURL
}

func (ytm *YoutubeMedia) FileURLExpiresAt() *time.Time {
	return ytm.StreamExpiresAt
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

func (ytm *YoutubeMedia) EnsureLoaded() error {
	if ytm.StreamURL == "" || (ytm.StreamExpiresAt != nil && time.Since(*ytm.StreamExpiresAt) > -5*time.Minute) {
		media, err := ytm.ytAPI.GetYoutubeMedia(ytm.Link)

		if err != nil {
			return err
		}

		ytm.StreamURL = media.StreamURL
		ytm.StreamExpiresAt = media.StreamExpiresAt
	}

	return nil
}

// Verify implements entities.Media
var _ entities.Media = (*YoutubeMedia)(nil)
