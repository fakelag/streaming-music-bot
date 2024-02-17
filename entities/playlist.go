package entities

import "errors"

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
	ConsumeNextMedia() (Media, error)
	SetConsumeOrder(PlaylistConsumeOrder) error
	SetRemoveOnConsume(removeOnConsume bool)
	GetAvailableConsumeOrders() []PlaylistConsumeOrder
	GetMediaCount() int
	GetRemoveOnConsume() bool
	GetConsumeOrder() PlaylistConsumeOrder
}
