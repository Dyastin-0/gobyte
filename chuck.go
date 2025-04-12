package gobyte

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/google/uuid"
)

func (c *Client) chuck(peer *Peer, files []FileInfo) {
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

	c.transferMU.Lock()
	c.pendingTransfers[transferID] = ackChan
	c.transferMU.Unlock()

	defer func() {
		conn.Close()
		c.transferMU.Lock()
		delete(c.pendingTransfers, transferID)
		c.transferMU.Unlock()
	}()

	transferReq := Message{
		Type:       TypeTransferReq,
		SenderID:   c.self.ID,
		SenderName: c.self.Name,
		IPAddress:  c.self.IPAddress,
		Files:      files,
		TransferID: transferID,
	}

	transferReqBytes, err := json.Marshal(transferReq)
	if err != nil {
		fmt.Printf("Error marshaling transfer request: %v\n", err)
		return
	}

	_, err = conn.Write(transferReqBytes)
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

		tcpConn, err := net.DialTimeout("tcp", tcpAddr, 15*time.Second)
		if err != nil {
			fmt.Println(ERROR.Render(err.Error()))
			return
		}
		defer tcpConn.Close()

		writer := bufio.NewWriter(tcpConn)
		defer writer.Flush()

		for _, fileInfo := range files {
			if err := chuck(fileInfo, writer); err != nil {
				fmt.Println(ERROR.Render(fmt.Sprintf("failed to send %s: %v", fileInfo.Name, err)))
			}
		}

		writer.WriteString("END\n")
		writer.Flush()
		fmt.Println(SUCCESS.Render(fmt.Sprintf("All files sent to %s ✓", peer.Name)))

	case <-time.After(15 * time.Second):
		fmt.Println(ERROR.Render(fmt.Sprintf("Timeout waiting for %s to accept the transfer.", peer.Name)))
		return
	}
}

func chuck(fileInfo FileInfo, writer *bufio.Writer) error {
	fmt.Println(INFO.Render(fmt.Sprintf("Sending %s...", fileInfo.Name)))
	file, err := os.Open(fileInfo.Path)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}

	header := fmt.Sprintf("FILE:%s:%d\n", fileInfo.Name, fileInfo.Size)
	if _, err = writer.WriteString(header); err != nil {
		file.Close()
		return fmt.Errorf("error sending file header: %v", err)
	}
	writer.Flush()

	sent, err := io.CopyN(writer, file, fileInfo.Size)
	if err != nil {
		file.Close()
		return fmt.Errorf("error sending file data: %v", err)
	}

	file.Close()
	fmt.Println(SUCCESS.Render(fmt.Sprintf("%s sent (%d bytes)", fileInfo.Name, sent)))

	return nil
}
