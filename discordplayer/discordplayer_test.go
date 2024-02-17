package discordplayer_test

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/fakelag/dca"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/fakelag/streaming-music-bot/discordplayer"
	. "github.com/fakelag/streaming-music-bot/discordplayer/mocks"
	"github.com/fakelag/streaming-music-bot/entities"
)

type MockMedia struct {
	DisableJumpToTS bool
	FileURLExpireAt *time.Time
}

func (mm *MockMedia) Title() string {
	return "Mock Media"
}

func (mm *MockMedia) FileURL() string {
	return "mockurl"
}

func (mm *MockMedia) CanJumpToTimeStamp() bool {
	return !mm.DisableJumpToTS
}

func (mm *MockMedia) Duration() *time.Duration {
	oneMinute := 1 * time.Minute
	return &oneMinute
}

func (mm *MockMedia) EnsureLoaded() error {
	return nil
}

func (mm *MockMedia) FileURLExpiresAt() *time.Time {
	return mm.FileURLExpireAt
}

func (mm *MockMedia) Thumbnail() string {
	return ""
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
	ctx context.Context,
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

	dms, err := discordplayer.NewDiscordMusicSessionEx(ctx, mockDca, mockDiscordSession, &discordplayer.DiscordMusicSessionOptions{
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
	return JoinMockVoiceChannelAndPlayEx(context.TODO(), ctrl, currentMediaDone, true, mockDcaStreamingSession)
}

var _ = Describe("Discord Player", func() {
	It("Creates a music session and starts playing media after enqueueing it", func() {
		ctrl := gomock.NewController(GinkgoT())

		currentMediaDone := make(chan error)
		playerContext := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone)

		c := make(chan struct{})

		go func() {
			// "play" music for 1 second and send done signal
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

		It("Returns an error from Leave() before the bot has joined & after it has left", func() {
			ctrl := gomock.NewController(GinkgoT())

			currentMediaDone := make(chan error)
			playerContext := JoinMockVoiceChannelAndPlayEx(context.TODO(), ctrl, currentMediaDone, false, nil)
			playerContext.mockVoiceConnection.EXPECT().Speaking(false).MaxTimes(2)

			c := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(1)

			Expect(playerContext.dms.Leave()).To(MatchError(discordplayer.ErrorWorkerNotActive))

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
				Expect(playerContext.dms.Leave()).To(MatchError(discordplayer.ErrorWorkerNotActive))
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
				Expect(playerContext.dms.Skip()).To(MatchError(discordplayer.ErrorWorkerNotActive))
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
			playerContext := JoinMockVoiceChannelAndPlayEx(context.TODO(), ctrl, currentMediaDone, true, mockDcaStreamingSession)

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

		It("Jumps to a timestamp in the currently playing media upon receiving the jump command", func() {
			ctrl := gomock.NewController(GinkgoT())

			currentMediaDone := make(chan error)
			playerContext := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone)

			jumpToTimeStamp := 30 * time.Second
			playbackPositionAtReload := 0
			c := make(chan struct{})

			gomock.InOrder(
				playerContext.mockVoiceConnection.EXPECT().Speaking(false),
				playerContext.mockVoiceConnection.EXPECT().IsReady().Return(true),
				playerContext.mockVoiceConnection.EXPECT().Speaking(true),
				playerContext.mockDca.EXPECT().EncodeFile(playerContext.mockMedia.FileURL(), gomock.Any()).
					Return(nil, nil).
					Do(func(path string, encodeOptions *dca.EncodeOptions) {
						playbackPositionAtReload = encodeOptions.StartTime
						playerContext.dms.Leave()
					}),
				playerContext.mockVoiceConnection.EXPECT().Speaking(false),
				playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
					close(c)
				}),
			)

			Eventually(func() entities.Media {
				return playerContext.dms.GetCurrentlyPlayingMedia()
			}).WithTimeout(5 * time.Second).WithPolling(50 * time.Millisecond).ShouldNot(BeNil())

			Expect(playerContext.dms.Jump(jumpToTimeStamp)).To(Succeed())

			select {
			case <-c:
				Expect(playbackPositionAtReload).To(BeNumerically("~", 30, 1))
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})

		It("Returns sensible errors from jump command if jumping to the given TS is not possible", func() {
			ctrl := gomock.NewController(GinkgoT())

			currentMediaDone := make(chan error)
			playerContext := JoinMockVoiceChannelAndPlay(ctrl, currentMediaDone)

			c := make(chan struct{})

			gomock.InOrder(
				playerContext.mockVoiceConnection.EXPECT().Speaking(false),
				playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
					close(c)
				}),
			)

			Eventually(func() entities.Media {
				return playerContext.dms.GetCurrentlyPlayingMedia()
			}).WithTimeout(5 * time.Second).WithPolling(50 * time.Millisecond).ShouldNot(BeNil())

			Expect(playerContext.dms.Jump(-1 * time.Second)).To(MatchError(discordplayer.ErrorInvalidArgument))
			playerContext.mockMedia.DisableJumpToTS = true
			Expect(playerContext.dms.Jump(30 * time.Second)).To(MatchError(discordplayer.ErrorMediaUnsupportedFeature))
			playerContext.mockMedia.DisableJumpToTS = false
			Expect(playerContext.dms.Jump(70 * time.Second)).To(MatchError(discordplayer.ErrorInvalidArgument))

			Expect(playerContext.dms.Leave()).To(Succeed())

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

			playerContext := JoinMockVoiceChannelAndPlayEx(context.TODO(), ctrl, nil, false, nil)

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
			playerContext := JoinMockVoiceChannelAndPlayEx(context.TODO(), ctrl, currentMediaDone, true, mockDcaStreamingSession)

			playerContext.mockMedia.DisableJumpToTS = disableReloadFromTS

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
				if playerContext.mockMedia.DisableJumpToTS {
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
			playerContext := JoinMockVoiceChannelAndPlayEx(context.TODO(), ctrl, currentMediaDone, false, mockDcaStreamingSession)

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

	When("Current media FileURL is about to expire", func() {
		It("Reloads the player from timestamp before FileURL expires", func() {
			ctrl := gomock.NewController(GinkgoT())

			currentMediaDone := make(chan error)
			mockDcaStreamingSession := NewMockDcaStreamingSession(ctrl)
			playerContext := JoinMockVoiceChannelAndPlayEx(context.TODO(), ctrl, currentMediaDone, false, mockDcaStreamingSession)

			expireInFewSeconds := time.Now().Add(2 * time.Second)
			playerContext.mockMedia.FileURLExpireAt = &expireInFewSeconds

			playerContext.mockVoiceConnection.EXPECT().Speaking(gomock.Any()).AnyTimes()
			playerContext.mockVoiceConnection.EXPECT().IsReady().Return(true).AnyTimes()

			Expect(playerContext.dms.EnqueueMedia(playerContext.mockMedia)).To(Succeed())

			c := make(chan struct{})

			encoderCalledWithStartTime := 0

			gomock.InOrder(
				mockDcaStreamingSession.EXPECT().PlaybackPosition().Return(10*time.Second).MinTimes(1),
				playerContext.mockDca.EXPECT().EncodeFile(playerContext.mockMedia.FileURL(), gomock.Any()).
					Return(nil, nil).
					Do(func(path string, encodeOptions *dca.EncodeOptions) {
						// Update expiration
						expireInOneHour := time.Now().Add(1 * time.Hour)
						playerContext.mockMedia.FileURLExpireAt = &expireInOneHour

						encoderCalledWithStartTime = encodeOptions.StartTime
						playerContext.dms.Leave()
					}),
				playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
					close(c)
				}),
			)

			select {
			case <-c:
				Expect(encoderCalledWithStartTime).To(BeNumerically("~", 10, 1))
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})
	})

	When("Giving a custom context to DiscordMusicSession", func() {
		It("Stops the worker and exits gracefully when top-level context is canceled when actively playing media", func() {
			ctx, cancel := context.WithCancel(context.TODO())
			ctrl := gomock.NewController(GinkgoT())

			currentMediaDone := make(chan error)
			mockDcaStreamingSession := NewMockDcaStreamingSession(ctrl)
			playerContext := JoinMockVoiceChannelAndPlayEx(ctx, ctrl, currentMediaDone, true, mockDcaStreamingSession)

			playerContext.mockVoiceConnection.EXPECT().IsReady().Return(true).AnyTimes()

			c := make(chan struct{})

			gomock.InOrder(
				playerContext.mockVoiceConnection.EXPECT().Speaking(false),
				playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
					close(c)
				}),
			)

			go func() {
				time.Sleep(1 * time.Second)
				cancel()
			}()

			select {
			case <-c:
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})

		It("Stops the worker and exits gracefully when top-level context is canceled when not actively playing", func() {
			ctx, cancel := context.WithCancel(context.TODO())
			ctrl := gomock.NewController(GinkgoT())

			currentMediaDone := make(chan error)
			mockDcaStreamingSession := NewMockDcaStreamingSession(ctrl)
			playerContext := JoinMockVoiceChannelAndPlayEx(ctx, ctrl, currentMediaDone, true, mockDcaStreamingSession)

			playerContext.mockVoiceConnection.EXPECT().Speaking(gomock.Any()).AnyTimes()
			playerContext.mockVoiceConnection.EXPECT().IsReady().Return(true).AnyTimes()

			c := make(chan struct{})

			playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
				close(c)
			})

			go func() {
				currentMediaDone <- nil
				time.Sleep(2 * time.Second)
				cancel()
			}()

			select {
			case <-c:
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})
	})

	When("Adding custom callbacks to DiscordMusicSession", func() {
		It("Invokes next media callbacks as expected", func() {
			ctrl := gomock.NewController(GinkgoT())

			currentMediaDone := make(chan error)
			mockDcaStreamingSession := NewMockDcaStreamingSession(ctrl)
			playerContext := JoinMockVoiceChannelAndPlayEx(context.TODO(), ctrl, currentMediaDone, false, mockDcaStreamingSession)

			isSecondCall := false
			playerContext.dms.AddNextMediaCallback(func(mediaFile entities.Media, isReload bool) {
				defer GinkgoRecover()
				Expect(mediaFile).NotTo(BeNil())
				Expect(mediaFile.FileURL()).To(Equal(playerContext.mockMedia.FileURL()))
				Expect(isReload).To(Equal(isSecondCall))

				if isReload {
					Expect(playerContext.dms.Leave()).To(Succeed())
				}

				isSecondCall = true
			})

			c := make(chan struct{})
			mockDcaStreamingSession.EXPECT().PlaybackPosition().Return(1 * time.Second).AnyTimes()
			playerContext.mockDca.EXPECT().EncodeFile(playerContext.mockMedia.FileURL(), gomock.Any()).Return(nil, nil).AnyTimes()
			playerContext.mockVoiceConnection.EXPECT().IsReady().Return(true).AnyTimes()
			playerContext.mockVoiceConnection.EXPECT().Speaking(gomock.Any()).AnyTimes()
			playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
				close(c)
			})

			playerContext.dms.EnqueueMedia(playerContext.mockMedia)

			// Error to force a reload
			currentMediaDone <- errors.New("Voice connection closed")

			select {
			case <-c:
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})

		It("Invokes error callbacks as expected", func() {
			ctrl := gomock.NewController(GinkgoT())
			mediaError := errors.New("ffmpeg exited with status code 1")

			currentMediaDone := make(chan error)
			mockDcaStreamingSession := NewMockDcaStreamingSession(ctrl)
			playerContext := JoinMockVoiceChannelAndPlayEx(context.TODO(), ctrl, currentMediaDone, false, mockDcaStreamingSession)

			playerContext.dms.AddErrorCallback(func(mediaFile entities.Media, err error) {
				defer GinkgoRecover()
				Expect(mediaFile).NotTo(BeNil())
				Expect(mediaFile.FileURL()).To(Equal(playerContext.mockMedia.FileURL()))
				Expect(err).To(MatchError(mediaError))
				Expect(playerContext.dms.Leave()).To(Succeed())
			})

			c := make(chan struct{})
			mockDcaStreamingSession.EXPECT().PlaybackPosition().Return(1 * time.Second).AnyTimes()
			playerContext.mockDca.EXPECT().EncodeFile(playerContext.mockMedia.FileURL(), gomock.Any()).Return(nil, nil).AnyTimes()
			playerContext.mockVoiceConnection.EXPECT().IsReady().Return(true).AnyTimes()
			playerContext.mockVoiceConnection.EXPECT().Speaking(gomock.Any()).AnyTimes()
			playerContext.mockVoiceConnection.EXPECT().Disconnect().Do(func() {
				close(c)
			})

			playerContext.dms.EnqueueMedia(playerContext.mockMedia)

			currentMediaDone <- mediaError

			select {
			case <-c:
				return
			case <-time.After(20 * time.Second):
				Fail("Voice worker timed out")
			}
		})
	})
})
