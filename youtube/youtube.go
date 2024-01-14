package youtube

import (
	"errors"
	"musicbot/cmd"
	"os/exec"
	"runtime"
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

	resultChannel, errorChannel := yt.executor.RunCommandWithTimeout(ytDlp, yt.streamUrlTimeout, "foo")

	var stdout *string

	select {
	case stdout = <-resultChannel:
		break
	case err := <-errorChannel:
		return "", err
	}

	if len(*stdout) == 0 {
		return "", errors.New("No video found or its too long")
	}

	return *stdout, nil
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
