package gobyte

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
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

func (c *Client) displayPeers() {
	c.MU.RLock()
	defer c.MU.RUnlock()

	if len(c.KnownPeers) == 0 {
		fmt.Println(INFO.Render("No peers found on the network."))
		return
	}

	fmt.Printf(INFO.Render("Found %d peers on the network:\n"), len(c.KnownPeers))
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("%-20s %-15s %s\n", "NAME", "IP ADDRESS", "ID")
	fmt.Println(strings.Repeat("-", 50))

	for _, peer := range c.KnownPeers {
		fmt.Printf("%-20s %-15s %s\n", peer.Name, peer.IPAddress, peer.ID)
	}
}
