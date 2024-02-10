package discordplayer_test

import (
	"errors"
	"sync"
	"time"

	"github.com/fakelag/dca"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"musicbot/discordplayer"
	. "musicbot/discordplayer/mocks"
	"musicbot/entities"
)

type MockMedia struct {
	DisableReloadFromTS bool
}

func (mm *MockMedia) FileURL() string {
	return "mockurl"
}

func (mm *MockMedia) CanReloadFromTimeStamp() bool {
	return !mm.DisableReloadFromTS
}

func (mm *MockMedia) Duration() *time.Duration {
	oneMinute := 1 * time.Minute
	return &oneMinute
}

type JoinVoiceAndPlayContext struct {
	mockVoiceConnection *MockDiscordVoiceConnection
	mockDiscordSession  *MockDiscordSession
	mockDca             *MockDiscordAudio
	dms                 *discordplayer.DiscordMusicSession
	mockMedia           *MockMedia
	guildID             string
	channelID           string
}

func JoinMockVoiceChannelAndPlayEx(
	ctrl *gomock.Controller,
	currentMediaDone chan error,
	enqueueMedia bool,
	streamingSession *MockDcaStreamingSession,
) *JoinVoiceAndPlayContext {
	mockDca := NewMockDiscordAudio(ctrl)
	mockDiscordSession := NewMockDiscordSession(ctrl)
	mockVoiceConnection := NewMockDiscordVoiceConnection(ctrl)

	gID := "xxx-guild-id"
	cID := "xxx-channel-id"
	mockMedia := &MockMedia{}

	gomock.InOrder(
		mockDiscordSession.EXPECT().ChannelVoiceJoin(gID, cID, false, false).Return(mockVoiceConnection, nil),
		mockVoiceConnection.EXPECT().Speaking(true),
	)

	mockDca.EXPECT().EncodeFile(mockMedia.FileURL(), gomock.Any()).Return(nil, nil).Times(1)
	mockDca.EXPECT().NewStream(nil, mockVoiceConnection, gomock.Any()).Return(streamingSession).MinTimes(1).
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

	return &JoinVoiceAndPlayContext{
		mockVoiceConnection: mockVoiceConnection,
		mockDiscordSession:  mockDiscordSession,
		mockDca:             mockDca,
		dms:                 dms,
		mockMedia:           mockMedia,
		guildID:             gID,
		channelID:           cID,
	}
}

func JoinMockVoiceChannelAndPlay(
	ctrl *gomock.Controller,
	currentMediaDone chan error,
) *JoinVoiceAndPlayContext {
	mockDcaStreamingSession := NewMockDcaStreamingSession(ctrl)
	return JoinMockVoiceChannelAndPlayEx(ctrl, currentMediaDone, true, mockDcaStreamingSession)
}

var _ = Describe("Playing music on a voice channel", func() {
	It("Creates a music session and starts playing media after enqueueing it", func() {
		ctrl := gomock.NewController(GinkgoT())

		currentMediaDone := make(chan error)
		playerContext := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone)

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

		playerContext.mockVoiceConnection.EXPECT().Speaking(false).Do(func(b bool) {
			wg.Done()
		})

		select {
		case <-c:
			return
		case <-time.After(20 * time.Second):
			Fail("Voice worker timed out")
		}
	})

	When("The voice worker is asked to execute commands through the API", func() {
		It("Starts playing and leaves upon receiving leave command", func() {
			ctrl := gomock.NewController(GinkgoT())

			currentMediaDone := make(chan error)
			playerContext := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone)
			// Leave() will call Speaking(false) if leaving during playing
			playerContext.mockVoiceConnection.EXPECT().Speaking(false).MaxTimes(1)

			c := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(1)

			go func() {
				time.Sleep(1 * time.Second)
				Expect(playerContext.dms.Leave()).To(Succeed())

				wg.Wait()
				close(c)
			}()

			playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
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
			playerContext := JoinMockVoiceChannelAndPlayEx(ctrl, currentMediaDone, false, nil)
			playerContext.mockVoiceConnection.EXPECT().Speaking(false).MaxTimes(2)

			c := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(1)

			Expect(playerContext.dms.Leave()).To(MatchError("voice worker inactive"))

			playerContext.dms.EnqueueMedia(playerContext.mockMedia)

			go func() {
				time.Sleep(1 * time.Second)
				Expect(playerContext.dms.Leave()).To(Succeed())

				wg.Wait()
				close(c)
			}()

			playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
				close(currentMediaDone)
				wg.Done()
			})

			select {
			case <-c:
				Expect(playerContext.dms.Leave()).To(MatchError("voice worker inactive"))
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})

		It("Starts playing and skips the current media upon receiving skip command", func() {
			ctrl := gomock.NewController(GinkgoT())

			currentMediaDone := make(chan error)
			playerContext := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone)
			playerContext.mockVoiceConnection.EXPECT().Speaking(gomock.Any()).AnyTimes()

			Eventually(func() entities.Media {
				return playerContext.dms.GetCurrentlyPlayingMedia()
			}).WithTimeout(2 * time.Second).WithPolling(50 * time.Millisecond).ShouldNot(BeNil())

			Expect(playerContext.dms.Skip()).To(Succeed())

			Eventually(func() entities.Media {
				return playerContext.dms.GetCurrentlyPlayingMedia()
			}).WithTimeout(2 * time.Second).WithPolling(50 * time.Millisecond).Should(BeNil())

			disconnectChannel := make(chan struct{})
			playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
				close(currentMediaDone)
				close(disconnectChannel)
			})

			Expect(playerContext.dms.Leave()).To(Succeed())

			select {
			case <-disconnectChannel:
				Expect(playerContext.dms.Skip()).To(MatchError("voice worker inactive"))
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})

		It("Repeats the current media upon receiving repeat command", func() {
			ctrl := gomock.NewController(GinkgoT())

			currentMediaDone := make(chan error)
			playerContext := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone)
			playerContext.mockVoiceConnection.EXPECT().Speaking(gomock.Any()).AnyTimes()
			playerContext.mockVoiceConnection.EXPECT().IsReady().Return(true).AnyTimes()
			playerContext.mockDca.EXPECT().EncodeFile(playerContext.mockMedia.FileURL(), gomock.Any()).Return(nil, nil).AnyTimes()

			// Done to current media
			currentMediaDone <- nil

			Eventually(func() entities.Media {
				return playerContext.dms.GetCurrentlyPlayingMedia()
			}).WithTimeout(5 * time.Second).WithPolling(50 * time.Millisecond).Should(BeNil())

			err := playerContext.dms.Repeat()
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() entities.Media {
				return playerContext.dms.GetCurrentlyPlayingMedia()
			}).WithTimeout(5 * time.Second).WithPolling(50 * time.Millisecond).ShouldNot(BeNil())

			err = playerContext.dms.Repeat()
			Expect(err).NotTo(HaveOccurred())

			// Done done to first repeat
			currentMediaDone <- nil

			// Done to second repeat
			currentMediaDone <- nil

			Eventually(func() entities.Media {
				return playerContext.dms.GetCurrentlyPlayingMedia()
			}).WithTimeout(5 * time.Second).WithPolling(50 * time.Millisecond).Should(BeNil())

			c := make(chan struct{})

			playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
				close(currentMediaDone)
				close(c)
			})

			Expect(playerContext.dms.Leave()).To(Succeed())

			select {
			case <-c:
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})

		It("Pauses & unpauses the current music streaming session upon receiving the pause command", func() {
			ctrl := gomock.NewController(GinkgoT())

			currentMediaDone := make(chan error)
			mockDcaStreamingSession := NewMockDcaStreamingSession(ctrl)
			playerContext := JoinMockVoiceChannelAndPlayEx(ctrl, currentMediaDone, true, mockDcaStreamingSession)

			mockIsPaused := false
			mockDcaStreamingSession.EXPECT().SetPaused(gomock.Any()).Do(func(pause bool) {
				mockIsPaused = pause
			}).Times(2)
			mockDcaStreamingSession.EXPECT().Paused().DoAndReturn(func() bool {
				return mockIsPaused
			}).MinTimes(1)

			c := make(chan struct{})

			Eventually(func() error {
				_, err := playerContext.dms.IsPaused()
				return err
			}).WithPolling(50 * time.Millisecond).WithTimeout(1 * time.Second).Should(Succeed())

			isPaused, err := playerContext.dms.IsPaused()
			Expect(err).NotTo(HaveOccurred())
			Expect(isPaused).To(BeFalse())

			Expect(playerContext.dms.SetPaused(true)).To(Succeed())

			isPaused, err = playerContext.dms.IsPaused()
			Expect(err).NotTo(HaveOccurred())
			Expect(isPaused).To(BeTrue())

			Expect(playerContext.dms.SetPaused(false)).To(Succeed())

			isPaused, err = playerContext.dms.IsPaused()
			Expect(err).NotTo(HaveOccurred())
			Expect(isPaused).To(BeFalse())

			go func() {
				time.Sleep(1 * time.Second)
				currentMediaDone <- nil
				close(currentMediaDone)
			}()

			playerContext.mockVoiceConnection.EXPECT().Speaking(false).Do(func(b bool) {
				close(c)
			})

			select {
			case <-c:
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
			playerContext := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone)
			Expect(playerContext.dms.EnqueueMedia(playerContext.mockMedia)).NotTo(HaveOccurred()) // Enqueue a second media

			c := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(2)

			playerContext.mockVoiceConnection.EXPECT().IsReady().Return(true).AnyTimes()
			playerContext.mockVoiceConnection.EXPECT().Speaking(true).MaxTimes(2)
			playerContext.mockDca.EXPECT().EncodeFile(playerContext.mockMedia.FileURL(), gomock.Any()).Return(nil, nil).MaxTimes(2)
			playerContext.mockVoiceConnection.EXPECT().Speaking(false).Do(func(b bool) {
				wg.Done()
			}).MaxTimes(2)

			Eventually(func() int {
				return len(playerContext.dms.GetMediaQueue())
			}).
				WithTimeout(5 * time.Second).
				WithPolling(50 * time.Millisecond).
				Should(Equal(1))

			Eventually(func() entities.Media {
				return playerContext.dms.GetCurrentlyPlayingMedia()
			}).
				WithTimeout(2 * time.Second).
				WithPolling(50 * time.Millisecond).
				Should(Not(BeNil()))

			// Done first media
			currentMediaDone <- nil

			Eventually(func() int {
				return len(playerContext.dms.GetMediaQueue())
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
					return playerContext.dms.GetCurrentlyPlayingMedia()
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
			playerContext := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone)
			Expect(playerContext.dms.EnqueueMedia(playerContext.mockMedia)).NotTo(HaveOccurred())
			Expect(playerContext.dms.EnqueueMedia(playerContext.mockMedia)).NotTo(HaveOccurred())

			c := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(1)

			playerContext.mockVoiceConnection.EXPECT().IsReady().Return(true).AnyTimes()
			playerContext.mockVoiceConnection.EXPECT().Speaking(gomock.Any()).MaxTimes(2)
			playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
				wg.Done()
			})

			Eventually(func() int {
				return len(playerContext.dms.GetMediaQueue())
			}).
				WithTimeout(5 * time.Second).
				WithPolling(50 * time.Millisecond).
				Should(Equal(2))

			Expect(playerContext.dms.ClearMediaQueue()).To(BeTrue())
			Expect(playerContext.dms.GetMediaQueue()).To(HaveLen(0))
			Expect(playerContext.dms.ClearMediaQueue()).To(BeFalse())

			Expect(playerContext.dms.Leave()).To(Succeed())

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

			playerContext := JoinMockVoiceChannelAndPlayEx(ctrl, nil, false, nil)

			// Enqueue 10 media
			for index := 0; index < 10; index += 1 {
				Expect(playerContext.dms.EnqueueMedia(playerContext.mockMedia)).NotTo(HaveOccurred())
			}

			Eventually(func() int {
				return len(playerContext.dms.GetMediaQueue())
			}).
				WithTimeout(5 * time.Second).
				WithPolling(50 * time.Millisecond).
				Should(Equal(9))

			Expect(playerContext.dms.EnqueueMedia(playerContext.mockMedia)).NotTo(HaveOccurred())
			Expect(playerContext.dms.EnqueueMedia(playerContext.mockMedia)).To(MatchError(discordplayer.ErrorMediaQueueFull))

			c := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(1)

			playerContext.mockVoiceConnection.EXPECT().IsReady().Return(true).AnyTimes()
			playerContext.mockVoiceConnection.EXPECT().Speaking(gomock.Any()).AnyTimes()
			playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
				wg.Done()
			})

			Expect(playerContext.dms.Leave()).To(Succeed())

			go func() {
				wg.Wait()
				close(c)
			}()

			select {
			case <-c:
				// Expect media queue to be empty after Leave()
				Eventually(func() int {
					return len(playerContext.dms.GetMediaQueue())
				}).
					WithTimeout(2 * time.Second).
					WithPolling(50 * time.Millisecond).
					Should(Equal(0))
				// Expect current media to be nil after Leave()
				Eventually(func() entities.Media {
					return playerContext.dms.GetCurrentlyPlayingMedia()
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

	When("Discord voice connection has a network error", func() {
		It("Reconnects to the voice if connection drops between playing sessions", func() {
			ctrl := gomock.NewController(GinkgoT())

			currentMediaDone := make(chan error)
			playerContext := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone)

			c := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(1)

			gomock.InOrder(
				playerContext.mockVoiceConnection.EXPECT().Speaking(false),
				playerContext.mockVoiceConnection.EXPECT().IsReady().Return(false),
				playerContext.mockDiscordSession.EXPECT().
					ChannelVoiceJoin(playerContext.guildID, playerContext.channelID, false, false).
					Return(playerContext.mockVoiceConnection, nil),
				playerContext.mockVoiceConnection.EXPECT().Speaking(true),
				playerContext.mockDca.EXPECT().EncodeFile(playerContext.mockMedia.FileURL(), gomock.Any()).Times(1),
				playerContext.mockVoiceConnection.EXPECT().Speaking(false),
				playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
					wg.Done()
				}),
			)

			// Done first media
			currentMediaDone <- nil

			playerContext.dms.EnqueueMedia(playerContext.mockMedia)

			// Done second media
			currentMediaDone <- nil

			Expect(playerContext.dms.Leave()).To(Succeed())

			go func() {
				wg.Wait()
				close(c)
				close(currentMediaDone)
			}()

			select {
			case <-c:
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})

		DescribeTable("Reconnects to the voice & resumes if discord connection drops while playing", func(
			disableReloadFromTS bool,
		) {
			ctrl := gomock.NewController(GinkgoT())

			mockDcaStreamingSession := NewMockDcaStreamingSession(ctrl)

			currentMediaDone := make(chan error)
			playerContext := JoinMockVoiceChannelAndPlayEx(ctrl, currentMediaDone, true, mockDcaStreamingSession)

			playerContext.mockMedia.DisableReloadFromTS = disableReloadFromTS

			c := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(1)

			encoderCalledWithStartTime := 0

			gomock.InOrder(
				playerContext.mockVoiceConnection.EXPECT().Speaking(false),
				mockDcaStreamingSession.EXPECT().PlaybackPosition().Return(10*time.Second).MinTimes(1),
				playerContext.mockVoiceConnection.EXPECT().IsReady().Return(false),
				playerContext.mockDiscordSession.EXPECT().
					ChannelVoiceJoin(playerContext.guildID, playerContext.channelID, false, false).
					Return(playerContext.mockVoiceConnection, nil),
				playerContext.mockVoiceConnection.EXPECT().Speaking(true),
				playerContext.mockDca.EXPECT().EncodeFile(playerContext.mockMedia.FileURL(), gomock.Any()).
					Return(nil, nil).
					Do(func(path string, encodeOptions *dca.EncodeOptions) {
						encoderCalledWithStartTime = encodeOptions.StartTime
					}),
				playerContext.mockVoiceConnection.EXPECT().Speaking(false),
				playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
					wg.Done()
				}),
			)

			// Error on first media
			currentMediaDone <- errors.New("Voice connection closed")

			// Done to first media after reload
			currentMediaDone <- nil

			Expect(playerContext.dms.Leave()).To(Succeed())

			go func() {
				wg.Wait()
				close(c)
				close(currentMediaDone)
			}()

			select {
			case <-c:
				if playerContext.mockMedia.DisableReloadFromTS {
					Expect(encoderCalledWithStartTime).To(Equal(0))
				} else {
					Expect(encoderCalledWithStartTime).To(BeNumerically("~", 10, 1))
				}
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		},
			Entry("Media can be resumed from current playback timestamp", false),
			Entry("Media does not support resuming from current playback timestamp", true),
		)

		It("Does not reconnect if a different error is encountered", func() {
			ctrl := gomock.NewController(GinkgoT())

			currentMediaDone := make(chan error)
			playerContext := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone)

			c := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(1)

			gomock.InOrder(
				playerContext.mockVoiceConnection.EXPECT().Speaking(false),
				playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
					wg.Done()
				}),
			)

			// Error on first media
			currentMediaDone <- errors.New("ffmpeg exited with status code 1")

			Expect(playerContext.dms.Leave()).To(Succeed())

			go func() {
				wg.Wait()
				close(c)
				close(currentMediaDone)
			}()

			select {
			case <-c:
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})
	})

	When("Requesting details about the current playing media through the API", func() {
		It("Returns media playback position if and only if currently playing media", func() {
			ctrl := gomock.NewController(GinkgoT())

			mockDcaStreamingSession := NewMockDcaStreamingSession(ctrl)

			currentMediaDone := make(chan error)
			playerContext := JoinMockVoiceChannelAndPlayEx(ctrl, currentMediaDone, false, mockDcaStreamingSession)

			Expect(playerContext.dms.CurrentPlaybackPosition()).To(Equal(time.Duration(0)))

			c := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(1)

			Expect(playerContext.dms.EnqueueMedia(playerContext.mockMedia)).NotTo(HaveOccurred())

			mockDcaStreamingSession.EXPECT().PlaybackPosition().Return(10 * time.Second).MinTimes(1)
			playerContext.mockVoiceConnection.EXPECT().Speaking(gomock.Any()).AnyTimes()
			playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
				wg.Done()
			})

			Eventually(func() time.Duration {
				return playerContext.dms.CurrentPlaybackPosition()
			}).WithTimeout(2 * time.Second).WithPolling(50 * time.Millisecond).Should(Equal(10 * time.Second))

			// Done playing
			currentMediaDone <- nil

			Eventually(func() time.Duration {
				return playerContext.dms.CurrentPlaybackPosition()
			}).WithTimeout(2 * time.Second).WithPolling(50 * time.Millisecond).Should(Equal(0 * time.Second))

			Expect(playerContext.dms.Leave()).To(Succeed())

			Eventually(func() time.Duration {
				return playerContext.dms.CurrentPlaybackPosition()
			}).WithTimeout(2 * time.Second).WithPolling(50 * time.Millisecond).Should(Equal(0 * time.Second))

			go func() {
				wg.Wait()
				close(c)
				close(currentMediaDone)
			}()

			select {
			case <-c:
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})
	})
})
