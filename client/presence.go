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

func (c *Client) presenceBroadcaster(ctx context.Context) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", c.broadcastAddr, c.discoveryPort))
	if err != nil {
		fmt.Println(styles.ERROR.Render(fmt.Sprintf("failed to resolve broadcast addr: %v", err)))
		return
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Println(styles.ERROR.Render(fmt.Sprintf("failed to dial broadcast addr: %v", err)))
		return
	}
	defer conn.Close()

	c.broadcastSelf(conn)

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.broadcastSelf(conn)
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) newPresenceMessage() ([]byte, error) {
	msg := types.Message{
		Type:       types.TypeUDPreq,
		SenderID:   c.Self.ID,
		SenderName: c.Self.Name,
		IPAddress:  c.Self.IPAddress,
	}

	return json.Marshal(msg)
}

func (c *Client) broadcastSelf(w interface{ Write([]byte) (int, error) }) {
	messageBytes, err := c.newPresenceMessage()
	if err != nil {
		fmt.Println(styles.ERROR.Render(fmt.Sprintf("failed to marshal broadcast message: %v", err)))
		return
	}

	_, err = w.Write(messageBytes)
	if err != nil {
		fmt.Println(styles.ERROR.Render(fmt.Sprintf("failed to send broadcast message: %v", err)))
	}
}
