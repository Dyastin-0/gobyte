package client

import "time"

const (
	DiscoverPort           = 8888
	TransferPort           = 8889
	BroadcastAddr          = "255.255.255.255"
	DiscoveryMessage       = "GOBYTE"
	MaxBuffer              = 1024 * 1024
	RequestTimeOutDuration = 15 * time.Second
)
