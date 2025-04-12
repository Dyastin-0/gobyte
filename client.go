package gobyte

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/google/uuid"
)

type Client struct {
	self             *Peer
	hostname         string
	knownPeers       map[string]*Peer
	mu               sync.RWMutex
	selectedFiles    []FileInfo
	transferReqChan  chan Message
	pendingTransfers map[string]chan bool
	transferMU       sync.RWMutex
	pendingPong      map[string]chan bool
	pongMU           sync.RWMutex
}

func NewClient(ctx context.Context) *Client {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}
	return &Client{
		self: &Peer{
			ID:        fmt.Sprintf("%s-%s", hostname, uuid.New().String()),
			Name:      hostname,
			IPAddress: getLocalIP(),
		},
		knownPeers:       make(map[string]*Peer),
		selectedFiles:    make([]FileInfo, 0),
		transferReqChan:  make(chan Message, 10),
		pendingTransfers: make(map[string]chan bool),
		pendingPong:      make(map[string]chan bool),
	}
}

func (c *Client) Run(ctx context.Context, cancel context.CancelFunc) {
	app := c.createCLIApp(cancel)

	fmt.Println(TITLE.Render("gobyte"))

	if err := app.Run(ctx, os.Args); err != nil {
		panic(err)
	}
}
