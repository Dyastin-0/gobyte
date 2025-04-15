package client

import (
	"context"
	"os"
	"sync"

	"github.com/Dyastin-0/gobyte/types"
	"github.com/Dyastin-0/gobyte/utils"
	"github.com/google/uuid"
)

type Client struct {
	Self *types.Peer

	knownPeers map[string]*types.Peer
	mu         sync.RWMutex

	transferReqChan  chan types.Message
	pendingTransfers map[string]chan bool
	transferMU       sync.RWMutex

	pendingPong map[string]chan bool
	pongMU      sync.RWMutex

	discoveryPort int
	transferPort  int
	broadcastAddr string
	discoveryMsg  string
	maxBufferSize int64
}

func New(ctx context.Context) *Client {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}

	self := &types.Peer{
		ID:        uuid.New().String(),
		Name:      hostname,
		IPAddress: utils.GetLocalIP(),
	}

	client := &Client{
		Self: self,

		knownPeers:       make(map[string]*types.Peer),
		transferReqChan:  make(chan types.Message, 10),
		pendingTransfers: make(map[string]chan bool),
		pendingPong:      make(map[string]chan bool),

		discoveryPort: 8888,
		transferPort:  8889,
		broadcastAddr: "255.255.255.255",
		discoveryMsg:  "GOBYTE",
		maxBufferSize: 1024 * 1024,
	}

	go client.listen(ctx)
	go client.pingBroadcaster(ctx)

	return client
}

func (c *Client) CountKnownPeers() (int, map[string]*types.Peer) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.knownPeers), c.knownPeers
}
