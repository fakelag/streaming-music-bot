package youtubeapi_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/fakelag/streaming-music-bot/testutils"
	"github.com/fakelag/streaming-music-bot/youtubeapi"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func NewYoutubeMedia(ytAPI *youtubeapi.Youtube) *youtubeapi.YoutubeMedia {
	expireAt := time.Now().Add(10 * time.Minute)
	media := &youtubeapi.YoutubeMedia{
		ID:                "1",
		VideoTitle:        "Mock Media 1",
		VideoLink:         "foobar",
		VideoIsLiveStream: false,
		VideoDuration:     60 * time.Second,
		StreamURL:         "streamurl1",
		StreamExpiresAt:   &expireAt,
	}

	media.SetYtAPI(ytAPI)

	return media
}

var _ = Describe("YT Media", func() {
	var videoJson string = strings.ReplaceAll(`{
		"id": "1",
		"fulltitle": "Mock Media 1",
		"duration": 60,
		"thumbnail": "foo",
		"is_live": false,
		"_type": "video"
	}`, "\n", "")

	var expireTimeUnix int64 = 1706280000
	var streamUrl string = "https://manifest.googlevideo.com/api/manifest/hls_playlist/expire/" + fmt.Sprintf("%d", expireTimeUnix) + "/ei&foo=bar"

	When("Reloading a media file", func() {
		It("Reloads media when it is not loaded and calling EnsureLoaded", func() {
			mockExecutor := &testutils.MockCommandExecutor{
				MockStdoutResult: streamUrl + "\n" + videoJson,
			}

			yt := youtubeapi.NewYoutubeAPI()
			yt.SetCmdExecutor(mockExecutor)

			media := NewYoutubeMedia(yt)
			media.StreamExpiresAt = nil
			media.StreamURL = ""

			Expect(media.EnsureLoaded()).To(Succeed())
			Expect(media.FileURLExpiresAt()).NotTo(BeNil())
			Expect(*media.FileURLExpiresAt()).To(BeTemporally("~", time.Unix(expireTimeUnix, 0), time.Second))
			Expect(media.FileURL()).To(Equal(streamUrl))
			Expect(media.CanJumpToTimeStamp()).To(BeTrue())
		})

		It("Reloads media when it is loaded but expired and calling EnsureLoaded", func() {
			newExpireTime := expireTimeUnix + 10 // +10s
			newFileURL := "https://manifest.googlevideo.com/api/manifest/hls_playlist/expire/" + fmt.Sprintf("%d", newExpireTime) + "/ei&foo=bar"
			mockExecutor := &testutils.MockCommandExecutor{
				MockStdoutResult: newFileURL + "\n" + videoJson,
			}

			yt := youtubeapi.NewYoutubeAPI()
			yt.SetCmdExecutor(mockExecutor)

			media := NewYoutubeMedia(yt)

			expireAt := time.Now().Add(10 * time.Second)
			media.StreamExpiresAt = &expireAt
			media.StreamURL = streamUrl

			Expect(media.EnsureLoaded()).To(Succeed())

			Expect(media.FileURLExpiresAt()).NotTo(BeNil())
			Expect(*media.FileURLExpiresAt()).To(BeTemporally("~", time.Unix(newExpireTime, 0), time.Second))
			Expect(media.FileURL()).To(Equal(newFileURL))
		})

		It("Returns a sensible error when yt api fails during EnsureLoaded", func() {
			mockExecutor := &testutils.MockCommandExecutor{
				MockStdoutResult: streamUrl + "\n" + "{\"_type\": \"something\"}",
			}

			yt := youtubeapi.NewYoutubeAPI()
			yt.SetCmdExecutor(mockExecutor)

			media := NewYoutubeMedia(yt)
			media.StreamExpiresAt = nil
			media.StreamURL = ""

			Expect(media.EnsureLoaded()).To(MatchError(youtubeapi.ErrorUnrecognisedObject))
		})
	})
})
