package youtube

import (
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

var _ = Describe("Downloading from YT", func() {
	It("Downloads a video stream URL from Youtube", func() {
		mockExecutor := &MockCommandExecutor{
			MockStdoutResult: "url123",
		}

		yt := &Youtube{
			executor: mockExecutor,
		}

		url, err := yt.GetYoutubeStreamURL("foo")
		Expect(err).To(BeNil())
		Expect(url).To(Equal("url123"))
	})
})
