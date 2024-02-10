package entities

import "time"

type Media interface {
	FileURL() string
	CanJumpToTimeStamp() bool
	Duration() *time.Duration
}
