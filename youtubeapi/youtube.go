package youtubeapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	cmd "github.com/fakelag/streaming-music-bot/command"
	"github.com/fakelag/streaming-music-bot/entities"
)

var (
	ErrorUnrecognisedObject = errors.New("unrecognised object type")
	ErrorInvalidYtdlpData   = errors.New("invalid ytdlp data")
	ErrorNoVideoFound       = errors.New("no video found")
	ErrorNoPlaylistFound    = errors.New("no playlist found")
)

type YtDlpObject struct {
	// "playlist", "video"
	Type string `json:"_type"`
}

type YtDlpVideoFormat struct {
	FormatID       string  `json:"format_id"`
	Format         string  `json:"format"`
	Ext            string  `json:"ext"`
	Url            string  `json:"url"`
	Fps            float64 `json:"fps"`
	Resolution     string  `json:"resolution"`
	FileSize       int     `json:"filesize"`
	FileSizeApprox int     `json:"filesize_approx"`
}

type YtDlpVideo struct {
	YtDlpObject
	ID           string `json:"id"`
	Title        string `json:"fulltitle"`
	Duration     int    `json:"duration"`
	Thumbnail    string `json:"thumbnail"`
	IsLiveStream bool   `json:"is_live"`
}

type YtDlpVideoWithFormats struct {
	YtDlpVideo
	Formats []*YtDlpVideoFormat `json:"formats"`
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

func (yt *Youtube) SearchYoutubeMedia(numSearchResults int, videoIdOrSearchTerm string) ([]*YoutubeMedia, error) {
	ytDlp, err := getYtDlpPath()

	if err != nil {
		return nil, err
	}

	replacer := strings.NewReplacer(
		"\"", "",
		"'", "",
	)

	videoArg := fmt.Sprintf("ytsearch%d:%s", numSearchResults, replacer.Replace(videoIdOrSearchTerm))

	args := []string{
		videoArg,
		"--extract-audio",
		"--quiet",
		"--audio-format", "opus",
		"--ignore-errors",
		"--no-color",
		"--no-check-formats",
		"--max-downloads", "0",
		"-s",
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

	searchResults := make([]*YoutubeMedia, 0)

	if len(stdout) == 0 {
		return searchResults, nil
	}

	jsonLines := strings.Split(stdout, "\n")

	if len(jsonLines) < 2 {
		return searchResults, nil
	}

	for i := 0; i < len(jsonLines); i += 2 {
		videoStreamURL := jsonLines[i]

		if videoStreamURL == "" {
			continue
		}

		videoJson := jsonLines[i+1]

		if videoJson == "" {
			continue
		}

		var object YtDlpObject
		if err := json.Unmarshal([]byte(videoJson), &object); err != nil {
			return nil, err
		}

		if object.Type != "video" {
			continue
		}

		media, _, err := yt.getMediaOrPlaylistFromJsonAndStreamURL(&object, videoJson, videoStreamURL)

		if err != nil {
			return nil, err
		}

		if media == nil {
			continue
		}

		searchResults = append(searchResults, media)
	}

	return searchResults, nil
}

func (yt *Youtube) GetYoutubeMedia(videoIdOrSearchTerm string) (*YoutubeMedia, error) {
	ytDlp, err := getYtDlpPath()

	if err != nil {
		return nil, err
	}

	videoArg := videoIdOrSearchTerm

	videoID := getYoutubeUrlVideoId(videoIdOrSearchTerm)

	if videoID == "" {
		videoArg = "ytsearch:" + videoIdOrSearchTerm
	} else {
		videoArg = "https://www.youtube.com/watch?v=" + videoID
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
		return nil, ErrorNoVideoFound
	}

	urlAndJson := strings.Split(stdout, "\n")

	if len(urlAndJson) < 2 {
		return nil, ErrorInvalidYtdlpData
	}

	videoStreamURL := urlAndJson[0]
	videoJson := urlAndJson[1]

	var object YtDlpObject
	if err := json.Unmarshal([]byte(videoJson), &object); err != nil {
		return nil, err
	}

	if object.Type != "video" {
		return nil, ErrorUnrecognisedObject
	}

	media, _, err := yt.getMediaOrPlaylistFromJsonAndStreamURL(&object, videoJson, videoStreamURL)
	return media, err
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
		return nil, ErrorNoPlaylistFound
	}

	var object YtDlpObject
	if err := json.Unmarshal([]byte(playlistJson), &object); err != nil {
		return nil, err
	}

	if object.Type != "playlist" {
		return nil, ErrorUnrecognisedObject
	}

	_, playList, err := yt.getMediaOrPlaylistFromJsonAndStreamURL(&object, playlistJson, "")
	return playList, err
}

func (yt *Youtube) ListFormats(videoIdOrUrl string) ([]*YtDlpVideoFormat, error) {
	ytDlp, err := getYtDlpPath()

	if err != nil {
		return nil, err
	}

	videoArg := videoIdOrUrl
	videoID := getYoutubeUrlVideoId(videoIdOrUrl)

	if videoID == "" {
		videoArg = videoIdOrUrl
	} else {
		videoArg = "https://www.youtube.com/watch?v=" + videoID
	}

	replacer := strings.NewReplacer(
		"\"", "",
		"'", "",
	)

	args := []string{
		replacer.Replace(videoArg),
		"--playlist-end", "1",
		"--quiet",
		"--ignore-errors",
		"--no-color",
		"--max-downloads", "0",
		"-s",
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
		return nil, ErrorNoVideoFound
	}

	jsonLines := strings.Split(stdout, "\n")

	if len(jsonLines) < 1 {
		return nil, ErrorInvalidYtdlpData
	}

	videoJson := jsonLines[0]

	var object YtDlpObject
	if err := json.Unmarshal([]byte(videoJson), &object); err != nil {
		return nil, err
	}

	if object.Type != "video" {
		return nil, ErrorUnrecognisedObject
	}

	var videoWithFormats YtDlpVideoWithFormats
	if err := json.Unmarshal([]byte(videoJson), &videoWithFormats); err != nil {
		return nil, err
	}

	return videoWithFormats.Formats, nil
}

func NewYoutubePlaylist(
	playlistID string,
	playlistTitle string,
	playlistLink string,
	rng *rand.Rand,
	numEntries int,
	entries ...*YoutubeMedia,
) *YoutubePlaylist {
	pl := &YoutubePlaylist{
		ID:                   playlistID,
		PlaylistTitle:        playlistTitle,
		PlaylistLink:         playlistLink,
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

func getYoutubeUrlVideoId(urlString string) string {
	parsedUrl, err := url.Parse(urlString)

	if err != nil {
		return ""
	}

	if !strings.Contains(parsedUrl.Hostname(), "youtube") && !strings.Contains(parsedUrl.Hostname(), "youtu.be") {
		return ""
	}

	rx := regexp.MustCompile("^.*(?:(?:youtu\\.be\\/|v\\/|vi\\/|u\\/\\w\\/|embed\\/|shorts\\/)|(?:(?:watch)?\\?v(?:i)?=|\\&v(?:i)?=))([^#\\&\\?]*).*")

	results := rx.FindStringSubmatch(urlString)

	if len(results) >= 2 {
		return results[1]
	}

	return ""
}

func (yt *Youtube) getMediaOrPlaylistFromJsonAndStreamURL(
	object *YtDlpObject,
	ytdlpJson string,
	videoStreamURL string,
) (*YoutubeMedia, *YoutubePlaylist, error) {
	if object.Type == "video" {
		var ytDlpVideo YtDlpVideo
		if err := json.Unmarshal([]byte(ytdlpJson), &ytDlpVideo); err != nil {
			return nil, nil, err
		}

		media := &YoutubeMedia{
			ID:                ytDlpVideo.ID,
			VideoTitle:        ytDlpVideo.Title,
			VideoThumbnail:    ytDlpVideo.Thumbnail,
			VideoIsLiveStream: ytDlpVideo.IsLiveStream,
			VideoDuration:     time.Duration(ytDlpVideo.Duration) * time.Second,
			VideoLink:         "https://www.youtube.com/watch?v=" + ytDlpVideo.ID,
			StreamURL:         videoStreamURL,
			ytAPI:             yt,
		}

		streamExpireUnixSecondsMatch := yt.streamUrlExpireRegex.FindStringSubmatch(videoStreamURL)

		if len(streamExpireUnixSecondsMatch) >= 4 {
			unixSeconds, err := strconv.ParseInt(streamExpireUnixSecondsMatch[3], 10, 64)

			if err == nil && unixSeconds > 0 {
				expirationTime := time.Unix(unixSeconds, 0)
				media.StreamExpiresAt = &expirationTime
			}
		}

		return media, nil, nil
	} else if object.Type == "playlist" {
		var ytDlpPlaylist YtDlpPlayList
		if err := json.Unmarshal([]byte(ytdlpJson), &ytDlpPlaylist); err != nil {
			return nil, nil, err
		}

		rngSource := rand.NewSource(time.Now().Unix())
		rng := rand.New(rngSource)

		playList := NewYoutubePlaylist(ytDlpPlaylist.ID, ytDlpPlaylist.Title, ytDlpPlaylist.PlaylistURL, rng, len(ytDlpPlaylist.Entries))

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
				ID:                video.ID,
				VideoTitle:        video.Title,
				VideoThumbnail:    thumbnailUrl,
				VideoIsLiveStream: video.LiveStatus == "is_live",
				VideoDuration:     time.Duration(video.Duration) * time.Second,
				VideoLink:         "https://www.youtube.com/watch?v=" + video.ID,
				StreamURL:         "",
				ytAPI:             yt,
			}
		}

		return nil, playList, nil
	} else {
		return nil, nil, ErrorUnrecognisedObject
	}
}

func getYtDlpPath() (string, error) {
	path, err := exec.LookPath("yt-dlp")

	if err != nil {
		return "", err
	}

	return path, nil
}
