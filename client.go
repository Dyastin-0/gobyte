package gobyte

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

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
	Shutdown         chan os.Signal
}

func NewClient(ctx context.Context) *Client {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}

	self := &Peer{
		ID:        fmt.Sprintf("%s-%s", hostname, uuid.New().String()),
		Name:      hostname,
		IPAddress: getLocalIP(),
	}

	return &Client{
		self:             self,
		knownPeers:       make(map[string]*Peer),
		selectedFiles:    make([]FileInfo, 0),
		transferReqChan:  make(chan Message, 10),
		pendingTransfers: make(map[string]chan bool),
		pendingPong:      make(map[string]chan bool),
		Shutdown:         make(chan os.Signal, 1),
	}
}

func (c *Client) Run(ctx context.Context) {
	app := c.createCLIApp()

	signal.Notify(c.Shutdown, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println(TITLE.Render("gobyte"), SUCCESS.Render(fmt.Sprintf("as %s (%s)", c.self.Name, c.self.IPAddress)))

	if err := app.Run(ctx, os.Args); err != nil {
		panic(err)
	}
}
