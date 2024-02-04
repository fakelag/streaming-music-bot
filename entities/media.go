package entities

import "time"

type Media interface {
	FileURL() string
	CanReloadFromTimeStamp() bool
	Duration() *time.Duration
}
