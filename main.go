package main

import (
	"fmt"
	"musicbot/youtube"
)

func main() {
	yt := youtube.NewYoutubeAPI()
	url, err := yt.GetYoutubeStreamURL("foo")

	if err != nil {
		panic(err)
	}

	fmt.Printf("url=%s\n", url)
	return
}
