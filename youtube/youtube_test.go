package youtube

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type MockCommandExecutor struct {
	MockStdoutResult string
}

func (command *MockCommandExecutor) RunCommandWithTimeout(
	executable string,
	timeout time.Duration,
	args ...string,
) (chan *string, chan error) {
	resultChannel := make(chan *string, 1)
	errorChannel := make(chan error, 1)

	go func() {
		resultChannel <- &command.MockStdoutResult
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

		yt := &Youtube{
			executor: mockExecutor,
		}

		media, err := yt.GetYoutubeMedia("foo")
		Expect(err).To(BeNil())
		Expect(media.ID).To(Equal("123"))
		Expect(media.Title).To(Equal("Mock Title"))
		Expect(media.StreamURL).To(Equal("url123"))
	})
})
