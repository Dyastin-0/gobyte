package gobyte

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

func (c *Client) broadcastPresence(ctx context.Context) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", broadcastAddr, discoveryPort))
	if err != nil {
		fmt.Printf("Error resolving broadcast address: %v\n", err)
		return
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Printf("Error creating UDP connection: %v\n", err)
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

func (c *Client) broadcastSelf(conn *net.UDPConn) {
	msg := Message{
		Type:       TypeUDPreq,
		SenderID:   c.self.ID,
		SenderName: c.self.Name,
		IPAddress:  c.self.IPAddress,
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Error marshaling discovery message: %v\n", err)
		return
	}

	_, err = conn.Write(jsonData)
	if err != nil {
		fmt.Printf("Error sending discovery broadcast: %v\n", err)
	}
}
