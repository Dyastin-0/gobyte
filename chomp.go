package gobyte

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

func (c *Client) chomp(ctx context.Context, dir string) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", transferPort))
	if err != nil {
		fmt.Printf("Error setting up file receiver: %v\n", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			listener.Close()
			return
		case msg := <-c.transferReqChan:
			fmt.Println(INFO.Render(fmt.Sprintf("File chomping request from %s (%s)", msg.SenderName, msg.IPAddress)))
			str := "file"
			if len(msg.Files) > 1 {
				str += "s"
			}
			confirm := c.showConfirm(fmt.Sprintf("Accept %d %s from %s?", len(msg.Files), str, msg.SenderName))

			responseAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", msg.IPAddress, discoveryPort))
			if err != nil {
				fmt.Printf("Error resolving sender address: %v\n", err)
				continue
			}

			respConn, err := net.DialUDP("udp", nil, responseAddr)
			if err != nil {
				fmt.Printf("Error creating UDP connection: %v\n", err)
				continue
			}

			response := Message{
				Type:       TypeTransferAck,
				SenderID:   c.self.ID,
				SenderName: c.self.Name,
				IPAddress:  c.self.IPAddress,
				Accepted:   confirm,
				TransferID: msg.TransferID,
			}

			jsonResponse, err := json.Marshal(response)
			if err != nil {
				fmt.Printf("Error marshaling response: %v\n", err)
				respConn.Close()
				continue
			}

			_, err = respConn.Write(jsonResponse)
			if err != nil {
				fmt.Printf("Error sending response: %v\n", err)
				respConn.Close()
				continue
			}
			respConn.Close()

			if !confirm {
				fmt.Println(INFO.Render("Files rejected."))
				continue
			}

			listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Minute))
			fmt.Println(INFO.Render("Waiting for connection..."))

			go chomp(listener, msg, dir)
		}
	}
}
