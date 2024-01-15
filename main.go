package main

import (
	"fmt"
	"musicbot/youtube"
)

func main() {
	yt := youtube.NewYoutubeAPI()
	media, err := yt.GetYoutubeMedia("dQw4w9WgXcQ")

	if err != nil {
		panic(err)
	}

	fmt.Printf("media=%+v\n", media)
	return
}
