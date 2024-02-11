package entities

import "time"

type Media interface {
	// URL passed to ffmpeg. Could be a local opus file or a remote url
	FileURL() string
	// URL expiration time. Voice worker will call EnsureLoaded before the URL is
	// about to expire and start a new encoding session with the FileURL() afterwards. Return nil
	// for no expiration (such as with a local file)
	FileURLExpiresAt() *time.Time
	// Ensure that the current FileURL() is valid. Called before starting an encoding session
	EnsureLoaded() error

	CanJumpToTimeStamp() bool
	Duration() *time.Duration
}
