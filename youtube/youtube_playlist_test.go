package youtube

import (
	"math/rand"
	"musicbot/entities"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func NewPlaylistWithMedia() *YoutubePlaylist {
	rngSource := rand.NewSource(GinkgoRandomSeed())
	rng := rand.New(rngSource)

	media1 := &YoutubeMedia{
		ID:              "1",
		Title:           "Mock Media 1",
		IsLiveStream:    false,
		VideoDuration:   60 * time.Second,
		StreamURL:       "streamurl1",
		StreamExpiresAt: time.Now().Add(10 * time.Minute),
	}

	media2 := &YoutubeMedia{
		ID:              "2",
		Title:           "Mock Media 2",
		IsLiveStream:    true,
		VideoDuration:   0 * time.Second,
		StreamURL:       "streamurl2",
		StreamExpiresAt: time.Now().Add(10 * time.Minute),
	}

	return &YoutubePlaylist{
		ID:                   "3",
		Title:                "Mock Playlist",
		rng:                  rng,
		removeMediaOnConsume: true,
		consumeOrder:         entities.ConsumeOrderFromStart,
		mediaList:            []*YoutubeMedia{media1, media2},
	}
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
	})
})
