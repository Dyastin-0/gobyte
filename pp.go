package gobye

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"golang.org/x/net/context"
)

func (c *Client) handleNewPeer(msg Message) {
	peer := Peer{
		ID:        msg.SenderID,
		Name:      msg.SenderName,
		IPAddress: msg.IPAddress,
	}

	c.mu.Lock()
	c.knownPeers[peer.ID] = &peer
	c.mu.Unlock()
}

func (c *Client) pingBroadcaster(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			<-ticker.C
			for _, peer := range c.knownPeers {
				go c.sendPing(peer)
			}

		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) sendPing(peer *Peer) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peer.IPAddress, discoveryPort))
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

	pingMsg := Message{
		Type:     TypeUDPping,
		SenderID: c.self.ID,
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
		c.pongMU.Lock()
		delete(c.pendingPong, peer.ID)
		c.pongMU.Unlock()

		return

	case <-time.After(250 * time.Millisecond):
		c.pongMU.Lock()
		delete(c.pendingPong, peer.ID)
		c.pongMU.Unlock()

		c.mu.Lock()
		delete(c.knownPeers, peer.ID)
		c.mu.Unlock()

		return
	}
}

func (c *Client) sendPong(peer *Peer) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peer.IPAddress, discoveryPort))
	if err != nil {
		fmt.Println(ERROR.Render(fmt.Sprintf("failed to send pong: %v", err)))
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Println(ERROR.Render(fmt.Sprintf("failed to dial peer: %v", err)))
	}

	defer conn.Close()

	pongMsg := Message{
		Type:     TypeUDPpong,
		SenderID: c.self.ID,
	}

	pongMsgBytes, err := json.Marshal(pongMsg)
	if err != nil {
		fmt.Printf("failed to marshal pong: %v\n", err)
	}

	_, err = conn.Write(pongMsgBytes)
	if err != nil {
		fmt.Printf("failed to send pong: %v\n", err)
	}
}
