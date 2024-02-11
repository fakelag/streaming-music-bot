package entities

import "time"

type Media interface {
	FileURL() string
	EnsureLoaded() error
	CanJumpToTimeStamp() bool
	Duration() *time.Duration
}
