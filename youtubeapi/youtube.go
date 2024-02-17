package youtubeapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	cmd "github.com/fakelag/streaming-music-bot/command"
	"github.com/fakelag/streaming-music-bot/entities"
	"github.com/fakelag/streaming-music-bot/utils"
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

type YtDlpPlayListThumbnail struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

type YtDlpPlayListEntry struct {
	YtDlpObject
	ID         string                   `json:"id"`
	Title      string                   `json:"title"`
	Duration   int                      `json:"duration"`
	LiveStatus string                   `json:"live_status"`
	Thumbnails []YtDlpPlayListThumbnail `json:"thumbnails"`
}

type YtDlpPlayList struct {
	YtDlpObject
	ID            string                `json:"id"`
	Title         string                `json:"title"`
	PlaylistCount int                   `json:"playlist_count"`
	PlaylistURL   string                `json:"webpage_url"`
	Entries       []*YtDlpPlayListEntry `json:"entries"`
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

func (yt *Youtube) SetCmdExecutor(exec cmd.CommandExecutor) {
	yt.executor = exec
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
		"--print-json",
	}

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
			ID:             ytDlpVideo.ID,
			VideoTitle:     ytDlpVideo.Title,
			VideoThumbnail: ytDlpVideo.Thumbnail,
			IsLiveStream:   ytDlpVideo.IsLiveStream,
			StreamURL:      videoStreamURL,
			VideoDuration:  time.Duration(ytDlpVideo.Duration) * time.Second,
			Link:           "https://www.youtube.com/watch?v=" + ytDlpVideo.ID,
			ytAPI:          yt,
		}

		streamExpireUnixSecondsMatch := yt.streamUrlExpireRegex.FindStringSubmatch(videoStreamURL)

		if len(streamExpireUnixSecondsMatch) >= 4 {
			unixSeconds, err := strconv.ParseInt(streamExpireUnixSecondsMatch[3], 10, 64)

			if err == nil && unixSeconds > 0 {
				expirationTime := time.Unix(unixSeconds, 0)
				media.StreamExpiresAt = &expirationTime
			}
		}

		return media, nil
	} else {
		return nil, errors.New(fmt.Sprintf("Unrecognised object type %s", object.Type))
	}
}

func (yt *Youtube) GetYoutubePlaylist(playlistIdOrUrl string) (*YoutubePlaylist, error) {
	ytDlp, err := getYtDlpPath()

	if err != nil {
		return nil, err
	}

	replacer := strings.NewReplacer(
		"\"", "",
		"'", "",
	)

	args := []string{
		replacer.Replace(playlistIdOrUrl),
		"--quiet",
		"--audio-format", "opus",
		"--ignore-errors",
		"--no-color",
		"--no-check-formats",
		"--max-downloads", "0",
		"--dump-single-json",
		"--flat-playlist",
	}

	resultChannel, errorChannel := yt.executor.RunCommandWithTimeout(ytDlp, yt.streamUrlTimeout, args...)

	var playlistJson string

	select {
	case result := <-resultChannel:
		playlistJson = *result
		break
	case err := <-errorChannel:
		return nil, err
	}

	if len(playlistJson) == 0 {
		return nil, errors.New("No playlist found")
	}

	var object YtDlpObject
	if err := json.Unmarshal([]byte(playlistJson), &object); err != nil {
		return nil, err
	}

	if object.Type != "playlist" {
		return nil, errors.New(fmt.Sprintf("Unrecognised object type %s", object.Type))
	}

	var ytDlpPlaylist YtDlpPlayList
	if err := json.Unmarshal([]byte(playlistJson), &ytDlpPlaylist); err != nil {
		return nil, err
	}

	rngSource := rand.NewSource(time.Now().Unix())
	rng := rand.New(rngSource)

	playList := NewYoutubePlaylist(ytDlpPlaylist.ID, ytDlpPlaylist.Title, rng, len(ytDlpPlaylist.Entries))

	for index, video := range ytDlpPlaylist.Entries {
		thumbnailUrl := ""
		thumbnailWidth := 0

		for _, thumbnail := range video.Thumbnails {
			if thumbnail.Width > thumbnailWidth {
				thumbnailUrl = thumbnail.URL
				thumbnailWidth = thumbnail.Width
			}
		}

		playList.mediaList[index] = &YoutubeMedia{
			ID:             video.ID,
			VideoTitle:     video.Title,
			VideoThumbnail: thumbnailUrl,
			IsLiveStream:   video.LiveStatus == "is_live",
			StreamURL:      "",
			VideoDuration:  time.Duration(video.Duration) * time.Second,
			Link:           "https://www.youtube.com/watch?v=" + video.ID,
			ytAPI:          yt,
		}
	}

	return playList, nil
}

func NewYoutubePlaylist(
	playlistID string,
	playlistTitle string,
	rng *rand.Rand,
	numEntries int,
	entries ...*YoutubeMedia,
) *YoutubePlaylist {
	pl := &YoutubePlaylist{
		ID:                   playlistID,
		PlaylistTitle:        playlistTitle,
		rng:                  rng,
		removeMediaOnConsume: true,
		consumeOrder:         entities.ConsumeOrderFromStart,
		mediaList:            make([]*YoutubeMedia, numEntries),
	}

	for index, entry := range entries {
		pl.mediaList[index] = entry
	}

	return pl
}

func getYtDlpPath() (string, error) {
	path, err := exec.LookPath("yt-dlp")

	if err != nil {
		return "", err
	}

	return path, nil
}
