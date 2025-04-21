package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Dyastin-0/gobyte/styles"
	"github.com/Dyastin-0/gobyte/types"
)

func (c *Client) listen(ctx context.Context) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", c.discoveryPort))
	if err != nil {
		fmt.Println(styles.ERROR.Render(fmt.Sprintf("failed to resolve udp address: %v", err)))
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println(styles.ERROR.Render(fmt.Sprintf("failed to listening on udp: %v", err)))
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
					fmt.Println(styles.ERROR.Render(fmt.Sprintf("failed to read from udp: %v", err)))
				}
				continue
			}

			var msg types.Message
			if err := json.Unmarshal(buffer[:n], &msg); err != nil {
				fmt.Println(styles.ERROR.Render(fmt.Sprintf("failed to unmarshal message: %v", err)))
				continue
			}

			c.handleUDPMessage(msg)
		}
	}
}

func (c *Client) handleUDPMessage(msg types.Message) {
	if msg.SenderID == c.Self.ID {
		return
	}

	switch msg.Type {
	case types.TypeUDPreq:
		c.handleNewPeer(msg)

	case types.TypeUDPping:
		c.mu.RLock()
		peer, exists := c.knownPeers[msg.SenderID]
		c.mu.RUnlock()
		if exists {
			c.sendPong(peer)
		}

	case types.TypeUDPpong:
		c.pongMU.RLock()
		ch, exists := c.pendingPong[msg.SenderID]
		c.pongMU.RUnlock()
		if exists {
			ch <- true
		}

	case types.TypeTransferReq:
		if c.Busy {
			c.sendAck(msg, "busy", false)
			break
		}
		c.Busy = true
		c.transferReqChan <- msg

	case types.TypeTransferAck:
		c.transferMU.RLock()
		ch, exists := c.pendingTransfers[msg.TransferID]
		c.transferMU.RUnlock()
		if exists {
			ch <- msg
		}
	}
}

func (c *Client) handleNewPeer(msg types.Message) {
	peer := types.Peer{
		ID:        msg.SenderID,
		Name:      msg.SenderName,
		IPAddress: msg.IPAddress,
	}

	c.mu.Lock()
	c.knownPeers[peer.ID] = peer
	c.mu.Unlock()
}
