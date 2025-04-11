package gobyte

import (
	"encoding/json"
	"fmt"
	"net"
)

func (c *Client) handleDiscovery(msg Message, remoteAddr *net.UDPAddr, conn *net.UDPConn) {
	peer := Peer{
		ID:        msg.SenderID,
		Name:      msg.SenderName,
		IPAddress: msg.IPAddress,
	}

	c.MU.Lock()
	c.KnownPeers[peer.ID] = &peer
	c.MU.Unlock()

	ackMsg := Message{
		Type:       TypeDiscoveryAck,
		SenderID:   c.Self.ID,
		SenderName: c.Self.Name,
		IPAddress:  c.Self.IPAddress,
	}

	jsonData, err := json.Marshal(ackMsg)
	if err != nil {
		fmt.Printf("Error marshaling ack message: %v\n", err)
		return
	}

	_, err = conn.WriteToUDP(jsonData, remoteAddr)
	if err != nil {
		fmt.Printf("Error sending discovery ack: %v\n", err)
	}
}

func (c *Client) handleDiscoveryAck(msg Message) {
	peer := Peer{
		ID:        msg.SenderID,
		Name:      msg.SenderName,
		IPAddress: msg.IPAddress,
	}

	c.MU.Lock()
	c.KnownPeers[peer.ID] = &peer
	c.MU.Unlock()
}
