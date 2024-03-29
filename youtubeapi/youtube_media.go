package youtubeapi

import (
	"time"

	"github.com/fakelag/streaming-music-bot/entities"
)

type YoutubeMedia struct {
	ID                string
	VideoTitle        string
	VideoIsLiveStream bool
	VideoThumbnail    string
	VideoDuration     time.Duration
	VideoLink         string

	// TODO Lock for StreamURL & StreamExpiresAt
	StreamURL       string
	StreamExpiresAt *time.Time
	ytAPI           *Youtube
}

func (ytm *YoutubeMedia) Title() string {
	return ytm.VideoTitle
}

func (ytm *YoutubeMedia) Link() string {
	return ytm.VideoLink
}

func (ytm *YoutubeMedia) FileURL() string {
	return ytm.StreamURL
}

func (ytm *YoutubeMedia) FileURLExpiresAt() *time.Time {
	return ytm.StreamExpiresAt
}

func (ytm *YoutubeMedia) CanJumpToTimeStamp() bool {
	return !ytm.VideoIsLiveStream
}

func (ytm *YoutubeMedia) Thumbnail() string {
	return ytm.VideoThumbnail
}

func (ytm *YoutubeMedia) Duration() *time.Duration {
	if ytm.VideoIsLiveStream {
		return nil
	}
	return &ytm.VideoDuration
}

func (ytm *YoutubeMedia) EnsureLoaded() error {
	if ytm.StreamURL == "" || (ytm.StreamExpiresAt != nil && time.Since(*ytm.StreamExpiresAt) > -5*time.Minute) {
		media, err := ytm.ytAPI.GetYoutubeMedia(ytm.Link())

		if err != nil {
			return err
		}

		ytm.StreamURL = media.StreamURL
		ytm.StreamExpiresAt = media.StreamExpiresAt
	}

	return nil
}

func (ytm *YoutubeMedia) IsLiveStream() bool {
	return ytm.VideoIsLiveStream
}

func (ytm *YoutubeMedia) SetYtAPI(ytAPI *Youtube) {
	ytm.ytAPI = ytAPI
}

// Verify implements entities.Media
var _ entities.Media = (*YoutubeMedia)(nil)
