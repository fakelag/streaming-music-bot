package discordplayer_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"musicbot/discordplayer"
	. "musicbot/discordplayer/mocks"
	"musicbot/entities"
)

type MockMedia struct {
}

func (mm *MockMedia) FileURL() string {
	return "mockurl"
}

func JoinMockVoiceChannelAndPlay(ctrl *gomock.Controller, currentMediaDone chan error, enqueueMedia bool) (
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
	)

	mockDca.EXPECT().EncodeFile(mockMedia.FileURL(), gomock.Any()).Return(nil, nil).MinTimes(1)
	mockDca.EXPECT().NewStream(nil, mockVoiceConnection, gomock.Any()).Return(nil).MinTimes(1).
		Do(func(encoding interface{}, voiceConn interface{}, d chan error) {
			if currentMediaDone != nil {
				go func() {
					select {
					case signal := <-currentMediaDone:
						d <- signal
					}
				}()
			}
		})

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
		Expect(dms.EnqueueMedia(mockMedia)).NotTo(HaveOccurred())
	}

	return mockVoiceConnection, dms, mockMedia
}

var _ = Describe("Playing music on a voice channel", func() {
	It("Creates a music session and starts playing media after enqueueing it", func() {
		ctrl := gomock.NewController(GinkgoT())

		currentMediaDone := make(chan error)
		mockVoiceConnection, _, _ := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone, true)

		c := make(chan struct{})

		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			// "play" music for 1 second and send done signal
			time.Sleep(1 * time.Second)
			currentMediaDone <- nil
			close(currentMediaDone)

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

			currentMediaDone := make(chan error)
			mockVoiceConnection, dms, _ := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone, true)

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
				close(currentMediaDone)
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

			currentMediaDone := make(chan error)
			mockVoiceConnection, dms, mockMedia := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone, false)

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
				close(currentMediaDone)
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

	When("Queueing media in the media queue", func() {
		It("Enqueues media, consumes it and returns it from the API", func() {
			ctrl := gomock.NewController(GinkgoT())

			currentMediaDone := make(chan error)
			mockVoiceConnection, dms, mockMedia := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone, true)
			Expect(dms.EnqueueMedia(mockMedia)).NotTo(HaveOccurred()) // Enqueue a second media

			c := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(2)

			mockVoiceConnection.EXPECT().IsReady().Return(true).AnyTimes()
			mockVoiceConnection.EXPECT().Speaking(true).MaxTimes(2)
			mockVoiceConnection.EXPECT().Speaking(false).Do(func(b bool) {
				wg.Done()
			}).MaxTimes(2)

			Eventually(func() int {
				return len(dms.GetMediaQueue())
			}).
				WithTimeout(5 * time.Second).
				WithPolling(50 * time.Millisecond).
				Should(Equal(1))

			Eventually(func() entities.Media {
				return dms.GetCurrentlyPlayingMedia()
			}).
				WithTimeout(2 * time.Second).
				WithPolling(50 * time.Millisecond).
				Should(Not(BeNil()))

			// Done first media
			currentMediaDone <- nil

			Eventually(func() int {
				return len(dms.GetMediaQueue())
			}).
				WithTimeout(5 * time.Second).
				WithPolling(50 * time.Millisecond).
				Should(Equal(0))

			// Done second media
			currentMediaDone <- nil

			go func() {
				wg.Wait()
				close(c)
				close(currentMediaDone)
			}()

			select {
			case <-c:
				// Current media should be nil after playing is done
				Eventually(func() entities.Media {
					return dms.GetCurrentlyPlayingMedia()
				}).
					WithTimeout(2 * time.Second).
					WithPolling(50 * time.Millisecond).
					Should(BeNil())
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})

		It("Clears media queue with the API", func() {
			ctrl := gomock.NewController(GinkgoT())

			currentMediaDone := make(chan error)
			mockVoiceConnection, dms, mockMedia := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone, true)
			Expect(dms.EnqueueMedia(mockMedia)).NotTo(HaveOccurred())
			Expect(dms.EnqueueMedia(mockMedia)).NotTo(HaveOccurred())

			c := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(1)

			mockVoiceConnection.EXPECT().IsReady().Return(true).AnyTimes()
			mockVoiceConnection.EXPECT().Speaking(gomock.Any()).MaxTimes(2)
			mockVoiceConnection.EXPECT().Disconnect().Do(func() {
				wg.Done()
			})

			Eventually(func() int {
				return len(dms.GetMediaQueue())
			}).
				WithTimeout(5 * time.Second).
				WithPolling(50 * time.Millisecond).
				Should(Equal(2))

			Expect(dms.ClearMediaQueue()).To(BeTrue())
			Expect(dms.GetMediaQueue()).To(HaveLen(0))
			Expect(dms.ClearMediaQueue()).To(BeFalse())

			Expect(dms.Leave()).To(BeTrue())

			go func() {
				wg.Wait()
				close(c)
			}()

			select {
			case <-c:
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})

		It("Gives an error when enqueueing past max queue size", func() {
			ctrl := gomock.NewController(GinkgoT())

			mockVoiceConnection, dms, mockMedia := JoinMockVoiceChannelAndPlay(ctrl, nil, false)

			// Enqueue 10 media
			for index := 0; index < 10; index += 1 {
				Expect(dms.EnqueueMedia(mockMedia)).NotTo(HaveOccurred())
			}

			Eventually(func() int {
				return len(dms.GetMediaQueue())
			}).
				WithTimeout(5 * time.Second).
				WithPolling(50 * time.Millisecond).
				Should(Equal(9))

			Expect(dms.EnqueueMedia(mockMedia)).NotTo(HaveOccurred())
			Expect(dms.EnqueueMedia(mockMedia)).To(MatchError("queue full"))

			c := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(1)

			mockVoiceConnection.EXPECT().IsReady().Return(true).AnyTimes()
			mockVoiceConnection.EXPECT().Speaking(gomock.Any()).AnyTimes()
			mockVoiceConnection.EXPECT().Disconnect().Do(func() {
				wg.Done()
			})

			Expect(dms.Leave()).To(BeTrue())

			go func() {
				wg.Wait()
				close(c)
			}()

			select {
			case <-c:
				// Expect media queue to be empty after Leave()
				Eventually(func() int {
					return len(dms.GetMediaQueue())
				}).
					WithTimeout(2 * time.Second).
					WithPolling(50 * time.Millisecond).
					Should(Equal(0))
				// Expect current media to be nil after Leave()
				Eventually(func() entities.Media {
					return dms.GetCurrentlyPlayingMedia()
				}).
					WithTimeout(2 * time.Second).
					WithPolling(50 * time.Millisecond).
					Should(BeNil())
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})
	})
})
