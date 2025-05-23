package client

import (
	"context"
	"maps"
	"os"
	"sync"

	"github.com/Dyastin-0/gobyte/types"
	"github.com/Dyastin-0/gobyte/utils"
	"github.com/google/uuid"
)

type writeFilesFunc func(peer types.Peer, files []types.FileInfo) error

type Client struct {
	Self *types.Peer
	Busy bool

	knownPeers map[string]types.Peer
	mu         sync.RWMutex

	transferReqChan  chan types.Message
	pendingTransfers map[string]chan types.Message
	transferMU       sync.RWMutex

	pendingPong map[string]chan bool
	pongMU      sync.RWMutex

	discoveryPort int
	transferPort  int
	broadcastAddr string
	discoveryMsg  string
	maxBufferSize int64

	writeFilesFunc
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

		knownPeers:       make(map[string]types.Peer),
		transferReqChan:  make(chan types.Message, 1),
		pendingTransfers: make(map[string]chan types.Message),
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

func (c *Client) GetKnownPeers() (int, map[string]types.Peer) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.knownPeers), cloneMap(c.knownPeers)
}

func cloneMap[T any](m map[string]T) map[string]T {
	newMap := make(map[string]T, len(m))
	maps.Copy(newMap, m)
	return newMap
}
