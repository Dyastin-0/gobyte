package gobyte

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (c *Client) sendFilesTo(peer *Peer, files []FileInfo) {
	transferID := uuid.New().String()

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peer.IPAddress, discoveryPort))
	if err != nil {
		fmt.Printf("Error resolving peer address: %v\n", err)
		return
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Printf("Error creating UDP connection: %v\n", err)
		return
	}

	ackChan := make(chan bool)

	c.MU.Lock()
	if c.PendingTransfers == nil {
		c.PendingTransfers = make(map[string]chan bool)
	}
	c.PendingTransfers[transferID] = ackChan
	c.MU.Unlock()

	defer func() {
		conn.Close()
		c.MU.Lock()
		delete(c.PendingTransfers, transferID)
		c.MU.Unlock()
	}()

	msg := Message{
		Type:       TypeTransferReq,
		SenderID:   c.Self.ID,
		SenderName: c.Self.Name,
		IPAddress:  c.Self.IPAddress,
		Files:      files,
		TransferID: transferID,
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Error marshaling transfer request: %v\n", err)
		return
	}

	_, err = conn.Write(jsonData)
	if err != nil {
		fmt.Printf("Error sending transfer request: %v\n", err)
		return
	}

	fmt.Println(INFO.Render(fmt.Sprintf("Request sent to %s. Waiting for response...", peer.Name)))

	select {
	case accepted := <-ackChan:
		if !accepted {
			fmt.Println(ERROR.Render(fmt.Sprintf("%s rejected the files.", peer.Name)))
			return
		}
		fmt.Println(SUCCESS.Render(fmt.Sprintf("%s accepted the request.", peer.Name)))

		tcpAddr := fmt.Sprintf("%s:%d", peer.IPAddress, transferPort)
		tcpConn, err := net.DialTimeout("tcp", tcpAddr, time.Minute*60)
		if err != nil {
			fmt.Println(ERROR.Render(err.Error()))
			return
		}
		defer tcpConn.Close()

		writer := bufio.NewWriter(tcpConn)
		defer writer.Flush()

		for _, fileInfo := range files {
			fmt.Println(INFO.Render(fmt.Sprintf("Sending %s...", fileInfo.Name)))
			file, err := os.Open(fileInfo.Path)
			if err != nil {
				fmt.Printf("Error opening file: %v\n", err)
				continue
			}

			header := fmt.Sprintf("FILE:%s:%d\n", fileInfo.Name, fileInfo.Size)
			if _, err = writer.WriteString(header); err != nil {
				fmt.Printf("Error sending file header: %v\n", err)
				file.Close()
				continue
			}
			writer.Flush()

			sent, err := io.CopyN(writer, file, fileInfo.Size)
			if err != nil {
				fmt.Printf("Error sending file data: %v\n", err)
				file.Close()
				continue
			}
			file.Close()
			fmt.Println(SUCCESS.Render(fmt.Sprintf("%s sent (%d bytes)", fileInfo.Name, sent)))
		}

		writer.WriteString("END\n")
		writer.Flush()
		fmt.Println(SUCCESS.Render(fmt.Sprintf("All files sent to %s ✓", peer.Name)))

	case <-time.After(15 * time.Second):
		fmt.Println(ERROR.Render(fmt.Sprintf("Timeout waiting for %s to accept the transfer.", peer.Name)))
		return
	}
}

func (c *Client) handleChomping(ctx context.Context, downloadDir string) {
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
				SenderID:   c.Self.ID,
				SenderName: c.Self.Name,
				IPAddress:  c.Self.IPAddress,
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

			listener.(*net.TCPListener).SetDeadline(time.Now().Add(30 * time.Second))
			fmt.Println(INFO.Render("Waiting for connection..."))

			go func(listener net.Listener, msg Message) {
				conn, err := listener.Accept()
				if err != nil {
					fmt.Printf("Error accepting connection: %v\n", err)
					return
				}
				defer conn.Close()

				fmt.Println(INFO.Render("Connected. Receiving files..."))
				if err := os.MkdirAll(downloadDir, 0755); err != nil {
					fmt.Printf("Error creating downloads directory: %v\n", err)
					return
				}

				reader := bufio.NewReader(conn)
				for {
					header, err := reader.ReadString('\n')
					if err != nil {
						if err == io.EOF {
							break
						}
						fmt.Printf("Error reading header: %v\n", err)
						return
					}
					header = strings.TrimSpace(header)
					if header == "END" {
						break
					}
					if !strings.HasPrefix(header, "FILE:") {
						fmt.Printf("Invalid header format: %s\n", header)
						return
					}
					parts := strings.Split(header, ":")
					if len(parts) != 3 {
						fmt.Printf("Invalid header format: %s\n", header)
						return
					}
					fileName := parts[1]
					fileSize, err := strconv.ParseInt(parts[2], 10, 64)
					if err != nil {
						fmt.Printf("Invalid file size: %v\n", err)
						return
					}
					filePath := filepath.Join(downloadDir, fileName)
					if _, err = os.Stat(filePath); err == nil {
						base := filepath.Base(fileName)
						ext := filepath.Ext(base)
						name := strings.TrimSuffix(base, ext)
						filePath = filepath.Join(downloadDir, fmt.Sprintf("%s_%d%s", name, time.Now().Unix(), ext))
					}
					file, err := os.Create(filePath)
					if err != nil {
						fmt.Printf("Error creating file: %v\n", err)
						return
					}
					received, err := io.CopyN(file, reader, fileSize)
					if err != nil {
						fmt.Printf("Error receiving file data: %v\n", err)
						file.Close()
						return
					}
					file.Close()
					fmt.Println(SUCCESS.Render(fmt.Sprintf("Received %s (%d bytes)", fileName, received)))
				}
				fmt.Println(SUCCESS.Render("File chomping complete ✓"))
			}(listener, msg)
		}
	}
}

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
			n, remoteAddr, err := conn.ReadFromUDP(buffer)
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

			if msg.SenderID == c.Self.ID {
				continue
			}

			switch msg.Type {
			case TypeDiscovery:
				c.handleDiscovery(msg, remoteAddr, conn)
			case TypeDiscoveryAck:
				c.handleDiscoveryAck(msg)
			case TypeTransferReq:
				select {
				case c.transferReqChan <- msg:
				default:
					fmt.Printf("Warning: Transfer request channel full, dropping request from %s\n", msg.SenderName)
				}
			case TypeTransferAck:
				c.MU.RLock()
				ch, exists := c.PendingTransfers[msg.TransferID]
				c.MU.RUnlock()

				if exists {
					ch <- msg.Accepted
				}
			}
		}
	}
}

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

	c.sendDiscoveryBroadcast(conn)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.sendDiscoveryBroadcast(conn)
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) sendDiscoveryBroadcast(conn *net.UDPConn) {
	msg := Message{
		Type:       TypeDiscovery,
		SenderID:   c.Self.ID,
		SenderName: c.Self.Name,
		IPAddress:  c.Self.IPAddress,
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
