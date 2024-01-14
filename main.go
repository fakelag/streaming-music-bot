package main

import (
	"fmt"
	. "musicbot/youtube"
)

func main() {
	url := YTGetStreamUrl("foo")
	fmt.Printf("url=%s\n", url)
	return
}
