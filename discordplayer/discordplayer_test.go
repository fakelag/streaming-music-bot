package discordplayer_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"musicbot/discordplayer"
	. "musicbot/discordplayer/mocks"
)

type MockMedia struct {
}

func (mm *MockMedia) FileURL() string {
	return "mockurl"
}

func JoinMockVoiceChannelAndPlay(ctrl *gomock.Controller, done chan error, enqueueMedia bool) (
	*MockDiscordVoiceConnection,
	*discordplayer.DiscordMusicSession,
	*MockMedia,
) {
	mockDca := NewMockDiscordAudio(ctrl)
	mockDiscordSession := NewMockDiscordSession(ctrl)
	mockVoiceConnection := NewMockDiscordVoiceConnection(ctrl)

	gID := "xxx-guild-id"
	cID := "xxx-channel-id"
	mockMedia := &MockMedia{}

	gomock.InOrder(
		mockDiscordSession.EXPECT().ChannelVoiceJoin(gID, cID, false, false).Return(mockVoiceConnection, nil),
		mockVoiceConnection.EXPECT().IsReady().Return(true),
		mockVoiceConnection.EXPECT().Speaking(true),
		mockDca.EXPECT().EncodeFile(mockMedia.FileURL(), gomock.Any()).Return(nil, nil),
		mockDca.EXPECT().NewStream(nil, mockVoiceConnection, gomock.Any()).Return(nil).
			Do(func(encoding interface{}, voiceConn interface{}, d chan error) {
				go func() {
					select {
					case signal := <-done:
						d <- signal
					}
				}()
			}),
	)

	dms, err := discordplayer.NewDiscordMusicSessionEx(mockDca, mockDiscordSession, &discordplayer.DiscordMusicSessionOptions{
		GuildID:           gID,
		VoiceChannelID:    cID,
		MediaQueueMaxSize: 10,
	})

	if err != nil {
		panic(err)
	}

	Expect(dms).NotTo(BeNil())

	if enqueueMedia {
		dms.EnqueueMedia(mockMedia)
	}

	return mockVoiceConnection, dms, mockMedia
}

var _ = Describe("Playing music on a voice channel", func() {
	It("Creates a music session and starts playing media after enqueueing it", func() {
		ctrl := gomock.NewController(GinkgoT())

		done := make(chan error)
		mockVoiceConnection, _, _ := JoinMockVoiceChannelAndPlay(ctrl, done, true)

		c := make(chan struct{})

		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			// "play" music for 1 second and send done signal
			time.Sleep(1 * time.Second)
			done <- nil

			wg.Wait()
			close(c)
		}()

		mockVoiceConnection.EXPECT().Speaking(false).Do(func(b bool) {
			wg.Done()
		})

		select {
		case <-c:
			return
		case <-time.After(20 * time.Second):
			Fail("Voice worker timed out")
		}
	})

	When("The bot is asked to leave with Leave()", func() {
		It("Starts playing and immediately leaves upon receiving leave command", func() {
			ctrl := gomock.NewController(GinkgoT())

			done := make(chan error)
			mockVoiceConnection, dms, _ := JoinMockVoiceChannelAndPlay(ctrl, done, true)

			c := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(1)

			go func() {
				time.Sleep(1 * time.Second)
				Expect(dms.Leave()).To(BeTrue())

				wg.Wait()
				close(c)
			}()

			mockVoiceConnection.EXPECT().Disconnect().Do(func() {
				close(done)
				wg.Done()
			})

			select {
			case <-c:
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})

		It("Returns false from Leave() before the bot has joined & after it has left", func() {
			ctrl := gomock.NewController(GinkgoT())

			done := make(chan error)
			mockVoiceConnection, dms, mockMedia := JoinMockVoiceChannelAndPlay(ctrl, done, false)

			c := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(1)

			Expect(dms.Leave()).To(BeFalse())

			dms.EnqueueMedia(mockMedia)

			go func() {
				time.Sleep(1 * time.Second)
				Expect(dms.Leave()).To(BeTrue())

				wg.Wait()
				close(c)
			}()

			mockVoiceConnection.EXPECT().Disconnect().Do(func() {
				close(done)
				wg.Done()
			})

			select {
			case <-c:
				Expect(dms.Leave()).To(BeFalse())
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})
	})
})
