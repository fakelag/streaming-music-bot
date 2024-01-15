package youtube

import (
	"errors"
	cmd "musicbot/command"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Youtube struct {
	executor         cmd.CommandExecutor
	streamUrlTimeout time.Duration
}

func NewYoutubeAPI() *Youtube {
	yt := &Youtube{
		executor:         &cmd.DefaultCommandExecutor{},
		streamUrlTimeout: time.Second * 30,
	}

	return yt
}

func (yt *Youtube) GetYoutubeStreamURL(videoIdOrSearchTerm string) (string, error) {
	ytDlp, err := getYtDlpPath()

	if err != nil {
		return "", err
	}

	useSearch := true

	videoArg := videoIdOrSearchTerm

	if useSearch {
		videoArg = "ytsearch:" + videoIdOrSearchTerm
	}

	replacer := strings.NewReplacer(
		"\"", "",
		"'", "",
	)

	args := []string{
		replacer.Replace(videoArg),
		"--no-playlist",
		"--extract-audio",
		"--quiet",
		"--audio-format", "opus",
		"--ignore-errors",
		"--no-color",
		"--no-check-formats",
		"--max-downloads", "0",
		"--get-url",
		"--print-json", // TODO: Remove for playlists
	}

	// TODO: playlists
	// 		 --dump-single-json
	//       --playlist-end 1

	resultChannel, errorChannel := yt.executor.RunCommandWithTimeout(ytDlp, yt.streamUrlTimeout, args...)

	var stdout string

	select {
	case result := <-resultChannel:
		stdout = *result
		break
	case err := <-errorChannel:
		return "", err
	}

	if len(stdout) == 0 {
		return "", errors.New("No video found or its too long")
	}

	urlAndJson := strings.Split(stdout, "\n")
	url := urlAndJson[0]
	// TODO extract metadata
	// json := urlAndJson[1]

	// url expiration timestamp format
	// https://manifest.googlevideo.com/api/manifest/hls_playlist/expire/0000000000/ei/
	// https://rr5---sn-qo5-ixas.googlevideo.com/videoplayback?expire=0000000000&ei=

	return url, nil
}

func getYtDlpPath() (string, error) {
	if runtime.GOOS == "windows" {
		return "./yt-dlp.exe", nil
	}

	path, err := exec.LookPath("yt-dlp")

	if err != nil {
		return "", err
	}

	return path, nil
}
