package youtube

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fakelag/streaming-music-bot/entities"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type MockCommandExecutor struct {
	MockStdoutResult string
	MockExitCode     int
}

func (command *MockCommandExecutor) RunCommandWithTimeout(
	executable string,
	timeout time.Duration,
	args ...string,
) (chan *string, chan error) {
	resultChannel := make(chan *string, 1)
	errorChannel := make(chan error, 1)

	go func() {
		if command.MockExitCode != 0 {
			errorChannel <- errors.New(fmt.Sprintf("exit status %d", command.MockExitCode))
		} else {
			resultChannel <- &command.MockStdoutResult
		}
	}()

	return resultChannel, errorChannel
}

var mockVideoJson string = strings.ReplaceAll(`{
	"id": "123",
	"fulltitle": "Mock Title",
	"duration": 70,
	"thumbnail": "foo",
	"is_live": false,
	"_type": "video"
}`, "\n", "")

var mockPlaylistJson string = strings.ReplaceAll(`{
	"id": "1234",
	"title": "Playlist Title",
	"thumbnails": [
		{
			"url": "thumbnail123",
			"height": 64,
			"width": 64,
			"id": "0",
			"resolution": "64x64"
		}
	],
	"playlist_count": 2,
	"webpage_url": "playlist_url",
	"entries": [
		{
			"id": "0",
			"title": "foobar",
			"fulltitle": "foobar",
			"duration": 60,
			"live_status": "not_live",
			"thumbnails": [
				{
					"url": "thumbnail1234",
					"preference": 0,
					"id": "0"
				}
			]
		},
		{
			"id": "1",
			"title": "foobar 2 live",
			"fulltitle": "foobar 2 live",
			"duration": 0,
			"live_status": "is_live",
			"thumbnails": [
				{
					"url": "thumbnail1234",
					"preference": 0,
					"id": "0"
				}
			]
		}
	],
	"_type": "playlist"
}`, "\n", "")

var _ = Describe("YT Download", func() {
	When("Downloading a singular video", func() {
		It("Downloads a video stream URL from Youtube", func() {
			mockExecutor := &MockCommandExecutor{
				MockStdoutResult: "url123\n" + mockVideoJson,
			}

			yt := NewYoutubeAPI()
			yt.executor = mockExecutor

			media, err := yt.GetYoutubeMedia("foo")
			Expect(err).To(BeNil())
			Expect(media.ID).To(Equal("123"))
			Expect(media.Title()).To(Equal("Mock Title"))
			Expect(media.FileURL()).To(Equal("url123"))
			Expect(media.Thumbnail()).To(Equal("foo"))
		})

		It("Parses stream expiration url correctly", func() {
			var timeUnix int64 = 1706280000
			for _, streamUrl := range []string{
				"https://manifest.googlevideo.com/api/manifest/hls_playlist/expire/" + fmt.Sprintf("%d", timeUnix) + "/ei&foo=bar\n",
				"https://rr5---sn-qo5-ixas.googlevideo.com/videoplayback?expire=" + fmt.Sprintf("%d", timeUnix) + "&ei=123&foo=bar\n",
			} {
				mockExecutor := &MockCommandExecutor{
					MockStdoutResult: streamUrl + mockVideoJson,
				}

				yt := NewYoutubeAPI()
				yt.executor = mockExecutor

				media, err := yt.GetYoutubeMedia("foo")
				Expect(err).To(BeNil())
				Expect(mockExecutor.MockStdoutResult).To(ContainSubstring(media.StreamURL))
				Expect(media.StreamExpiresAt).NotTo(BeNil())
				Expect(*media.StreamExpiresAt).To(BeTemporally("~", time.Unix(timeUnix, 0), time.Second))
			}
		})

		It("Fails with a sensible error if the download fails with an exit code", func() {
			mockExecutor := &MockCommandExecutor{
				MockStdoutResult: "",
				MockExitCode:     1,
			}

			yt := NewYoutubeAPI()
			yt.executor = mockExecutor

			_, err := yt.GetYoutubeMedia("foo")
			Expect(err).To(MatchError("exit status 1"))
		})

		It("Fails with a sensible error if the download fails due to invalid json", func() {
			mockExecutor := &MockCommandExecutor{
				MockStdoutResult: "url123\n{",
			}

			yt := NewYoutubeAPI()
			yt.executor = mockExecutor

			_, err := yt.GetYoutubeMedia("foo")
			Expect(err).To(MatchError("unexpected end of JSON input"))
		})
	})

	When("Downloading a playlist", func() {
		It("Downloads a playlist & its videos", func() {
			mockExecutor := &MockCommandExecutor{
				MockStdoutResult: mockPlaylistJson,
			}

			yt := NewYoutubeAPI()
			yt.executor = mockExecutor

			playList, err := yt.GetYoutubePlaylist("foo")
			playList.SetConsumeOrder(entities.ConsumeOrderFromStart)

			Expect(err).To(BeNil())
			Expect(playList.ID).To(Equal("1234"))
			Expect(playList.Title).To(Equal("Playlist Title"))
			Expect(playList.GetMediaCount()).To(Equal(2))

			media0, err := playList.ConsumeNextMedia()
			Expect(err).To(BeNil())
			media1, err := playList.ConsumeNextMedia()
			Expect(err).To(BeNil())

			Expect(media0).Should(BeAssignableToTypeOf(&YoutubeMedia{}))
			Expect(media1).Should(BeAssignableToTypeOf(&YoutubeMedia{}))

			ytMedia0 := media0.(*YoutubeMedia)
			ytMedia1 := media1.(*YoutubeMedia)

			Expect(ytMedia0.ID).Should(Equal("0"))
			Expect(ytMedia0.Title()).Should(Equal("foobar"))
			Expect(ytMedia0.IsLiveStream).Should(BeFalse())
			Expect(ytMedia0.VideoDuration).Should(Equal(60 * time.Second))
			Expect(ytMedia0.StreamURL).Should(Equal(""))
			Expect(ytMedia0.Link).Should(Equal("https://www.youtube.com/watch?v=0"))

			Expect(ytMedia1.ID).Should(Equal("1"))
			Expect(ytMedia1.Title()).Should(Equal("foobar 2 live"))
			Expect(ytMedia1.IsLiveStream).Should(BeTrue())
			Expect(ytMedia1.VideoDuration).Should(Equal(0 * time.Second))
			Expect(ytMedia1.StreamURL).Should(Equal(""))
			Expect(ytMedia1.Link).Should(Equal("https://www.youtube.com/watch?v=1"))
		})

		It("Fails with a sensible error if the download fails due to invalid json", func() {
			mockExecutor := &MockCommandExecutor{
				MockStdoutResult: "{",
			}

			yt := NewYoutubeAPI()
			yt.executor = mockExecutor

			_, err := yt.GetYoutubePlaylist("foo")
			Expect(err).To(MatchError("unexpected end of JSON input"))
		})
	})
})
