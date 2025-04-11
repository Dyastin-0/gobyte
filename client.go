package gobyte

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/google/uuid"
)

type Client struct {
	Self             *Peer
	Hostname         string
	KnownPeers       map[string]*Peer
	SelectedFiles    []FileInfo
	MU               sync.RWMutex
	transferReqChan  chan Message
	PendingTransfers map[string]chan bool
}

func NewClient(ctx context.Context) *Client {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}
	return &Client{
		Self: &Peer{
			ID:        fmt.Sprintf("%s-%s", hostname, uuid.New().String()),
			Name:      hostname,
			IPAddress: getLocalIP(),
		},
		KnownPeers:       make(map[string]*Peer),
		SelectedFiles:    make([]FileInfo, 0),
		transferReqChan:  make(chan Message, 10),
		PendingTransfers: make(map[string]chan bool),
	}
}

func (c *Client) Run(ctx context.Context, cancel context.CancelFunc) {
	app := c.createCLIApp(cancel)
	if err := app.Run(ctx, os.Args); err != nil {
		panic(err)
	}
}
