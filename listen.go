package gobyte

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

func (c *Client) listen(ctx context.Context) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", discoveryPort))
	if err != nil {
		fmt.Printf("Error resolving UDP address: %v\n", err)
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Printf("Error listening on UDP: %v\n", err)
		return
	}
	defer conn.Close()

	buffer := make([]byte, 2048)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, _, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				if !strings.Contains(err.Error(), "i/o timeout") {
					fmt.Printf("Error reading from UDP: %v\n", err)
				}
				continue
			}

			var msg Message
			if err := json.Unmarshal(buffer[:n], &msg); err != nil {
				fmt.Printf("Error unmarshaling message: %v\n", err)
				continue
			}

			if msg.SenderID == c.self.ID {
				continue
			}

			switch msg.Type {
			case TypeUDPreq:
				c.handleNewPeer(msg)

			case TypeUDPping:
				c.mu.RLock()
				peer, exists := c.knownPeers[msg.SenderID]
				c.mu.RUnlock()

				if exists {
					c.sendPong(peer)
				}

			case TypeUDPpong:
				c.pongMU.RLock()
				ch, exists := c.pendingPong[msg.SenderID]
				c.pongMU.RUnlock()

				if exists {
					ch <- true
				}

			case TypeTransferReq:
				select {
				case c.transferReqChan <- msg:
				default:
					fmt.Printf("Warning: Transfer request channel full, dropping request from %s\n", msg.SenderName)
				}

			case TypeTransferAck:
				c.transferMU.RLock()
				ch, exists := c.pendingTransfers[msg.TransferID]
				c.transferMU.RUnlock()

				if exists {
					ch <- msg.Accepted
				}
			}
		}
	}
}
