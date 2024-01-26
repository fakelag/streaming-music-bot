package youtube

import (
	"errors"
	"fmt"
	"strings"
	"time"

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

var _ = Describe("Downloading from YT", func() {
	It("Downloads a video stream URL from Youtube", func() {
		mockExecutor := &MockCommandExecutor{
			MockStdoutResult: "url123\n" + mockVideoJson,
		}

		yt := NewYoutubeAPI()
		yt.executor = mockExecutor

		media, err := yt.GetYoutubeMedia("foo")
		Expect(err).To(BeNil())
		Expect(media.ID).To(Equal("123"))
		Expect(media.Title).To(Equal("Mock Title"))
		Expect(media.StreamURL).To(Equal("url123"))
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
			Expect(media.StreamExpiresAt).To(BeTemporally("~", time.Unix(timeUnix, 0), time.Second))
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
