package discordplayer_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"musicbot/discordplayer"
	. "musicbot/mock_discordplayer"
)

type MockMedia struct {
}

func (mm *MockMedia) FileURL() string {
	return "mockurl"
}

var _ = Describe("Playing music on a voice channel", func() {
	It("Creates a music session and starts playing media after enqueueing it", func() {
		ctrl := gomock.NewController(GinkgoT())

		mockDca := NewMockDiscordAudio(ctrl)
		mockDiscordSession := NewMockDiscordSession(ctrl)
		mockVoiceConnection := NewMockDiscordVoiceConnection(ctrl)

		gID := "xxx-guild-id"
		cID := "xxx-channel-id"
		mockMedia := &MockMedia{}

		c := make(chan struct{})

		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			wg.Wait()
			close(c)
		}()

		gomock.InOrder(
			mockDiscordSession.EXPECT().ChannelVoiceJoin(gID, cID, false, false).Return(mockVoiceConnection, nil),
			mockVoiceConnection.EXPECT().IsReady().Return(true),
			mockVoiceConnection.EXPECT().Speaking(true),
			mockDca.EXPECT().EncodeFile(mockMedia.FileURL(), gomock.Any()).Return(nil, nil),
			mockDca.EXPECT().NewStream(nil, mockVoiceConnection, gomock.Any()).Return(nil).
				Do(func(encoding interface{}, voiceConn interface{}, done chan error) {
					go func() {
						// "play" music for 1 second and send done signal
						time.Sleep(1 * time.Second)
						done <- nil
					}()
				}),
			mockVoiceConnection.EXPECT().Speaking(false).Do(func(b bool) {
				wg.Done()
			}),
		)

		dms, err := discordplayer.NewDiscordMusicSession(mockDca, mockDiscordSession, gID, cID)

		if err != nil {
			panic(err)
		}

		Expect(dms).NotTo(BeNil())

		dms.EnqueueMedia(mockMedia)

		select {
		case <-c:
			return
		case <-time.After(20 * time.Second):
			Fail("Voice worker timed out")
		}
	})
})
