package gobyte

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

func (c *Client) chuck(dir string) {
	peers, err := c.selectPeers()
	if err != nil || len(peers) == 0 {
		fmt.Println(INFO.Render("no peers to send to"))
		return
	}

	files, err := c.selectFiles(dir)
	if err != nil || len(files) == 0 {
		fmt.Println(INFO.Render(fmt.Sprintf("%v", err)))
		return
	}

	var wg sync.WaitGroup

	for _, peer := range peers {
		wg.Add(1)

		go func() {
			err := c.writeFiles(&peer, files, &wg)
			if err != nil {
				fmt.Println(ERROR.Render(fmt.Sprintf("%v", err)))
				return
			}

			fmt.Println(SUCCESS.Bold(true).Render(fmt.Sprintf("all files sent to %s ✓", peer.Name)))
		}()
	}

	wg.Wait()
}

func (c *Client) writeFiles(peer *Peer, files []FileInfo, wg *sync.WaitGroup) error {
	defer wg.Done()

	transferID := uuid.New().String()

	ackChan := make(chan bool)

	c.transferMU.Lock()
	c.pendingTransfers[transferID] = ackChan
	c.transferMU.Unlock()

	defer func() {
		c.transferMU.Lock()
		delete(c.pendingTransfers, transferID)
		c.transferMU.Unlock()
	}()

	err := c.sendTransferReq(peer, files, transferID)
	if err != nil {
		return err
	}

	fmt.Println(INFO.Render(fmt.Sprintf("request sent to %s. Waiting for response...", peer.Name)))

	select {
	case accepted := <-ackChan:
		if !accepted {
			return fmt.Errorf("%s rejected the request", peer.Name)
		}

		fmt.Println(SUCCESS.Render(fmt.Sprintf("%s accepted the request", peer.Name)))

		err := writeTo(peer, files)
		if err != nil {
			return err
		}

	case <-time.After(15 * time.Second):
		return fmt.Errorf("request for %s timed out", peer.Name)
	}

	return nil
}

func (c *Client) sendTransferReq(peer *Peer, files []FileInfo, transferID string) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peer.IPAddress, discoveryPort))
	if err != nil {
		return fmt.Errorf("failed to resolve addr")
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to dial udp")
	}

	defer conn.Close()

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
		return fmt.Errorf("failed to marshal transfer tansferReq")
	}

	_, err = conn.Write(transferReqBytes)
	if err != nil {
		return fmt.Errorf("failed to write transferReq")
	}

	return nil
}

func writeTo(peer *Peer, files []FileInfo) error {
	tcpAddr := fmt.Sprintf("%s:%d", peer.IPAddress, transferPort)

	tcpConn, err := net.DialTimeout("tcp", tcpAddr, 15*time.Second)
	if err != nil {
		return fmt.Errorf("failed to dial tcp: %v", err)
	}

	defer tcpConn.Close()

	writer := bufio.NewWriter(tcpConn)
	defer writer.Flush()

	for _, fileInfo := range files {
		if sentBytes, err := chuck(fileInfo, writer); err != nil {
			fmt.Println(ERROR.Render(fmt.Sprintf("failed to send %s: %v", fileInfo.Name, err)))
		} else {
			fmt.Println(SUCCESS.Render(fmt.Sprintf("%s sent (%d bytes)", fileInfo.Name, sentBytes)))
		}
	}

	writer.WriteString("END\n")
	writer.Flush()

	return nil
}

func chuck(fileInfo FileInfo, writer *bufio.Writer) (int64, error) {
	fmt.Println(INFO.Render(fmt.Sprintf("Sending %s...", fileInfo.Name)))
	file, err := os.Open(fileInfo.Path)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %v", err)
	}

	header := fmt.Sprintf("FILE:%s:%d\n", fileInfo.Name, fileInfo.Size)
	if _, err = writer.WriteString(header); err != nil {
		file.Close()
		return 0, fmt.Errorf("error sending file header: %v", err)
	}
	writer.Flush()

	sentBytes, err := io.CopyN(writer, file, fileInfo.Size)
	if err != nil {
		file.Close()

		return 0, fmt.Errorf("error sending file data: %v", err)
	}

	file.Close()

	return sentBytes, nil
}
