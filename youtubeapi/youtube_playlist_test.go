package youtubeapi_test

import (
	"math/rand"
	"time"

	"github.com/fakelag/streaming-music-bot/entities"
	"github.com/fakelag/streaming-music-bot/youtubeapi"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func NewPlaylistWithMedia() *youtubeapi.YoutubePlaylist {
	rngSource := rand.NewSource(GinkgoRandomSeed())
	rng := rand.New(rngSource)

	expireAt := time.Now().Add(10 * time.Minute)
	media1 := &youtubeapi.YoutubeMedia{
		ID:                "1",
		VideoTitle:        "Mock Media 1",
		VideoIsLiveStream: false,
		VideoDuration:     60 * time.Second,
		StreamURL:         "streamurl1",
		StreamExpiresAt:   &expireAt,
	}

	expireAt = time.Now().Add(10 * time.Minute)
	media2 := &youtubeapi.YoutubeMedia{
		ID:                "2",
		VideoTitle:        "Mock Media 2",
		VideoIsLiveStream: true,
		VideoDuration:     0 * time.Second,
		StreamURL:         "streamurl2",
		StreamExpiresAt:   &expireAt,
	}

	mediaList := []*youtubeapi.YoutubeMedia{media1, media2}
	return youtubeapi.NewYoutubePlaylist("3", "Mock Playlist", rng, len(mediaList), mediaList...)
}

var _ = Describe("YT Playlists", func() {
	When("Consuming media from a playlist", func() {
		It("Consumes media from the start & removes on consumption", func() {
			playList := NewPlaylistWithMedia()

			playList.SetConsumeOrder(entities.ConsumeOrderFromStart)
			playList.SetRemoveOnConsume(true)

			Expect(playList.GetRemoveOnConsume()).To(BeTrue())
			Expect(playList.GetConsumeOrder()).To(Equal(entities.ConsumeOrderFromStart))
			Expect(playList.GetMediaCount()).To(Equal(2))
			Expect(playList.GetDurationLeft()).NotTo(BeNil())
			Expect(*playList.GetDurationLeft()).To(Equal(60 * time.Second))

			media, err := playList.ConsumeNextMedia()
			Expect(err).ToNot(HaveOccurred())
			Expect(media).NotTo(BeNil())
			Expect(media.FileURL()).To(Equal("streamurl1"))
			Expect(playList.GetMediaCount()).To(Equal(1))

			media, err = playList.ConsumeNextMedia()
			Expect(err).ToNot(HaveOccurred())
			Expect(media).NotTo(BeNil())
			Expect(media.FileURL()).To(Equal("streamurl2"))
			Expect(playList.GetMediaCount()).To(Equal(0))

			media, err = playList.ConsumeNextMedia()
			Expect(err).To(MatchError(entities.ErrorPlaylistEmpty))
			Expect(media).To(BeNil())
		})

		It("Consumes media from the start without removal", func() {
			playList := NewPlaylistWithMedia()

			playList.SetConsumeOrder(entities.ConsumeOrderFromStart)
			playList.SetRemoveOnConsume(false)

			Expect(playList.GetRemoveOnConsume()).To(BeFalse())
			Expect(playList.GetConsumeOrder()).To(Equal(entities.ConsumeOrderFromStart))
			Expect(playList.GetMediaCount()).To(Equal(2))

			media, err := playList.ConsumeNextMedia()
			Expect(err).ToNot(HaveOccurred())
			Expect(media).NotTo(BeNil())
			Expect(media.FileURL()).To(Equal("streamurl1"))
			Expect(playList.GetMediaCount()).To(Equal(2))

			media, err = playList.ConsumeNextMedia()
			Expect(err).ToNot(HaveOccurred())
			Expect(media).NotTo(BeNil())
			Expect(media.FileURL()).To(Equal("streamurl2"))
			Expect(playList.GetMediaCount()).To(Equal(2))

			media, err = playList.ConsumeNextMedia()
			Expect(err).ToNot(HaveOccurred())
			Expect(media).NotTo(BeNil())
			Expect(media.FileURL()).To(Equal("streamurl1"))
			Expect(playList.GetMediaCount()).To(Equal(2))
		})

		It("Consumes media with shuffle & removes on consumption", func() {
			playList := NewPlaylistWithMedia()

			playList.SetConsumeOrder(entities.ConsumeOrderShuffle)
			playList.SetRemoveOnConsume(true)

			Expect(playList.GetRemoveOnConsume()).To(BeTrue())
			Expect(playList.GetConsumeOrder()).To(Equal(entities.ConsumeOrderShuffle))
			Expect(playList.GetMediaCount()).To(Equal(2))

			media1, err := playList.ConsumeNextMedia()
			Expect(err).ToNot(HaveOccurred())
			Expect(media1).NotTo(BeNil())
			Expect(playList.GetMediaCount()).To(Equal(1))

			media2, err := playList.ConsumeNextMedia()
			Expect(err).ToNot(HaveOccurred())
			Expect(media2).NotTo(BeNil())
			Expect(playList.GetMediaCount()).To(Equal(0))

			Expect(media1.FileURL()).ToNot(Equal(media2.FileURL()))

			media3, err := playList.ConsumeNextMedia()
			Expect(err).To(MatchError(entities.ErrorPlaylistEmpty))
			Expect(media3).To(BeNil())
		})

		It("Consumes media with shuffle without removal", func() {
			playList := NewPlaylistWithMedia()

			playList.SetConsumeOrder(entities.ConsumeOrderShuffle)
			playList.SetRemoveOnConsume(false)

			Expect(playList.GetRemoveOnConsume()).To(BeFalse())
			Expect(playList.GetConsumeOrder()).To(Equal(entities.ConsumeOrderShuffle))
			Expect(playList.GetMediaCount()).To(Equal(2))

			media, err := playList.ConsumeNextMedia()
			Expect(err).ToNot(HaveOccurred())
			Expect(media).NotTo(BeNil())
			Expect(playList.GetMediaCount()).To(Equal(2))
		})
	})
})
