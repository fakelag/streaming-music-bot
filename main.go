package main

import (
	"fmt"
	"musicbot/youtube"
)

func main() {
	yt := youtube.NewYoutubeAPI()
	url, err := yt.GetYoutubeStreamURL("dQw4w9WgXcQ")

	if err != nil {
		panic(err)
	}

	fmt.Printf("url=%s\n", url)
	return
}
