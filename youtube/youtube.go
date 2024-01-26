package youtube

import (
	"encoding/json"
	"errors"
	"fmt"
	cmd "musicbot/command"
	"musicbot/utils"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type YtDlpObject struct {
	// "playlist", "video"
	Type string `json:"_type"`
}

type YtDlpVideo struct {
	YtDlpObject
	ID           string `json:"id"`
	Title        string `json:"fulltitle"`
	Duration     int    `json:"duration"`
	Thumbnail    string `json:"thumbnail"`
	IsLiveStream bool   `json:"is_live"`
}

type Youtube struct {
	executor             cmd.CommandExecutor
	streamUrlTimeout     time.Duration
	streamUrlExpireRegex *regexp.Regexp
}

func NewYoutubeAPI() *Youtube {
	yt := &Youtube{
		executor:             &cmd.DefaultCommandExecutor{},
		streamUrlTimeout:     time.Second * 30,
		streamUrlExpireRegex: regexp.MustCompile("(expire)(\\/|=)(\\d+)(\\/|=|&|$)"),
	}

	return yt
}

func (yt *Youtube) GetYoutubeMedia(videoIdOrSearchTerm string) (*YoutubeMedia, error) {
	ytDlp, err := getYtDlpPath()

	if err != nil {
		return nil, err
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
		return nil, err
	}

	if len(stdout) == 0 {
		return nil, errors.New("No video found")
	}

	urlAndJson := strings.Split(stdout, "\n")

	if len(urlAndJson) < 2 {
		firstString := ""
		if len(urlAndJson) > 0 {
			firstString = urlAndJson[0]
		}
		return nil, errors.New(fmt.Sprintf("Invalid video json data: %s", utils.TruncateString(firstString, 50, "...")))
	}

	videoStreamURL := urlAndJson[0]
	videoJson := urlAndJson[1]

	var object YtDlpObject
	if err := json.Unmarshal([]byte(videoJson), &object); err != nil {
		return nil, err
	}

	if object.Type == "video" {
		var ytDlpVideo YtDlpVideo
		if err := json.Unmarshal([]byte(videoJson), &ytDlpVideo); err != nil {
			return nil, err
		}

		media := &YoutubeMedia{
			ID:        ytDlpVideo.ID,
			Title:     ytDlpVideo.Title,
			StreamURL: videoStreamURL,
		}

		streamExpireUnixSecondsMatch := yt.streamUrlExpireRegex.FindStringSubmatch(videoStreamURL)

		if len(streamExpireUnixSecondsMatch) >= 4 {
			unixSeconds, err := strconv.ParseInt(streamExpireUnixSecondsMatch[3], 10, 64)

			if err == nil && unixSeconds > 0 {
				media.StreamExpiresAt = time.Unix(unixSeconds, 0)
			}
		}

		return media, nil
	} else {
		return nil, errors.New(fmt.Sprintf("Unrecognised object type %s", object.Type))
	}
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
