package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/Dyastin-0/gobyte/styles"
	"github.com/Dyastin-0/gobyte/types"
)

func (c *Client) pingBroadcaster(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.RLock()
			for _, peer := range c.knownPeers {
				go c.sendPing(peer)
			}
			c.mu.RUnlock()

		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) sendPing(peer types.Peer) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peer.IPAddress, c.discoveryPort))
	if err != nil {
		fmt.Println("Failed to resolve address:", err)
		return
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Println("Failed to dial peer:", err)
		return
	}
	defer conn.Close()

	pingMsg := types.Message{
		Type:     types.TypeUDPping,
		SenderID: c.Self.ID,
	}

	pingMsgBytes, err := json.Marshal(pingMsg)
	if err != nil {
		fmt.Println("Failed to marshal ping:", err)
		return
	}
	_, err = conn.Write(pingMsgBytes)
	if err != nil {
		fmt.Println("Failed to send ping:", err)
		return
	}

	pongChan := make(chan bool)

	c.pongMU.Lock()
	c.pendingPong[peer.ID] = pongChan
	c.pongMU.Unlock()

	select {
	case <-pongChan:
		c.handlePingResponse(peer.ID, true)
	case <-time.After(250 * time.Millisecond):
		c.handlePingResponse(peer.ID, false)
	}
}

func (c *Client) handlePingResponse(peerID string, received bool) {
	c.pongMU.Lock()
	delete(c.pendingPong, peerID)
	c.pongMU.Unlock()

	if !received {
		c.mu.Lock()
		delete(c.knownPeers, peerID)
		c.mu.Unlock()
	}
}

func (c *Client) sendPong(peer types.Peer) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peer.IPAddress, c.discoveryPort))
	if err != nil {
		fmt.Println(styles.ERROR.Render(fmt.Sprintf("failed to send pong: %v", err)))
		return
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Println(styles.ERROR.Render(fmt.Sprintf("failed to dial peer: %v", err)))
		return
	}
	defer conn.Close()

	pongMsg := types.Message{
		Type:     types.TypeUDPpong,
		SenderID: c.Self.ID,
	}

	pongMsgBytes, err := json.Marshal(pongMsg)
	if err != nil {
		fmt.Printf("failed to marshal pong: %v\n", err)
		return
	}

	_, err = conn.Write(pongMsgBytes)
	if err != nil {
		fmt.Printf("failed to send pong: %v\n", err)
	}
}
