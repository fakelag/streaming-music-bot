package entities

import (
	"errors"
	"time"
)

type PlaylistConsumeOrder = string

const (
	ConsumeOrderFromStart PlaylistConsumeOrder = "start"
	ConsumeOrderShuffle   PlaylistConsumeOrder = "shuffle"
)

var (
	ErrorConsumeOrderNotSupported = errors.New("order not supported")
	ErrorPlaylistEmpty            = errors.New("playlist is empty")
)

type Playlist interface {
	Title() string
	Link() string
	ConsumeNextMedia() (Media, error)
	SetConsumeOrder(PlaylistConsumeOrder) error
	SetRemoveOnConsume(removeOnConsume bool)
	GetAvailableConsumeOrders() []PlaylistConsumeOrder
	GetMediaCount() int
	GetDurationLeft() *time.Duration
	GetRemoveOnConsume() bool
	GetConsumeOrder() PlaylistConsumeOrder
}
