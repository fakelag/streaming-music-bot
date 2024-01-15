package youtube

type YoutubeMedia struct {
	ID        string
	Title     string
	StreamURL string
}

// Verify implements entities.Media
// var _ entities.Media = YoutubeMedia{}
// var _ entities.Media = (*YoutubeMedia)(nil)
