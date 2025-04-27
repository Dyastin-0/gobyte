package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/Dyastin-0/gobyte/progress"
	"github.com/Dyastin-0/gobyte/styles"
	"github.com/Dyastin-0/gobyte/tofu"
	"github.com/Dyastin-0/gobyte/types"
	"github.com/google/uuid"
)

func (c *Client) ChuckFilesToPeers(peers []types.Peer, files []types.FileInfo) error {
	progress := progress.New()

	var wg sync.WaitGroup
	var sendErr error

	for _, peer := range peers {
		wg.Add(1)

		go func(p types.Peer) {
			defer wg.Done()

			err := c.writeFiles(p, files, progress)
			if err != nil {
				sendErr = err
				return
			}
		}(peer)
	}

	wg.Wait()
	progress.Wait()

	return sendErr
}

func (c *Client) writeFiles(peer types.Peer, files []types.FileInfo, p *progress.Progress) error {
	if c.writeFilesFunc != nil {
		return c.writeFilesFunc(peer, files)
	}

	transferID := uuid.New().String()

	ackChan := make(chan types.Message)

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

	select {
	case msg := <-ackChan:
		if !msg.Accepted && msg.Reason != "" {
			return fmt.Errorf("%s (%s) is %s", msg.SenderName, msg.IPAddress, msg.Reason)
		}

		if !msg.Accepted {
			return fmt.Errorf("%s rejected the request", peer.Name)
		}

		c.writeFilesToPeer(peer, files, p)

		return nil

	case <-time.After(15 * time.Second):
		return fmt.Errorf("request for %s timed out", peer.Name)
	}
}

func (c *Client) sendTransferReq(peer types.Peer, files []types.FileInfo, transferID string) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peer.IPAddress, c.discoveryPort))
	if err != nil {
		return fmt.Errorf("failed to resolve addr: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to dial udp: %v", err)
	}

	defer conn.Close()

	transferReq := types.Message{
		Type:       types.TypeTransferReq,
		SenderID:   c.Self.ID,
		SenderName: c.Self.Name,
		IPAddress:  c.Self.IPAddress,
		Files:      files,
		TransferID: transferID,
	}

	transferReqBytes, err := json.Marshal(transferReq)
	if err != nil {
		return fmt.Errorf("failed to marshal transfer tansferReq: %v", err)
	}

	_, err = conn.Write(transferReqBytes)
	if err != nil {
		return fmt.Errorf("failed to write transferReq: %v", err)
	}

	return nil
}

func (c *Client) writeFilesToPeer(peer types.Peer, files []types.FileInfo, p *progress.Progress) error {
	addr := fmt.Sprintf("%s:%d", peer.IPAddress, c.transferPort)

	homeDir, _ := os.UserHomeDir()
	certDir := fmt.Sprintf("%s/gobyte/cert", homeDir)
	trustDir := fmt.Sprintf("%s/gobyte/trust", homeDir)

	tofuID := fmt.Sprintf("%s (%s)", c.Self.Name, c.Self.IPAddress)

	tofu, err := tofu.New(tofuID, certDir, trustDir)
	if err != nil {
		return fmt.Errorf("failed to create tofu: %v", err)
	}

	conn, err := tofu.Connect(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", addr, err)
	}

	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(15 * time.Second))

	buffer := make([]byte, 1024)

	n, err := conn.Read(buffer)
	if err != nil {
		return err
	}

	message := string(buffer[:n])

	if message != "OK" {
		return fmt.Errorf("invalid message")
	}

	for _, fileInfo := range files {
		if _, err := copyN(conn, fileInfo, peer, p); err != nil {
			fmt.Println(styles.ERROR.Render(fmt.Sprintf("failed to chuck %s: %v", fileInfo.Name, err)))
		}
	}

	return nil
}

func copyN(conn io.Writer, fileInfo types.FileInfo, peer types.Peer, p *progress.Progress) (int64, error) {
	file, err := os.Open(fileInfo.Path)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %v", err)
	}

	defer file.Close()

	if err = writeFileHeader(conn, fileInfo); err != nil {
		return 0, fmt.Errorf("failed to write header: %v", err)
	}

	bar := p.NewBar(conn, file, fileInfo.Size, fmt.Sprintf("(%s %s) chucking %s...", peer.Name, peer.IPAddress, fileInfo.Name))

	sentBytes, err := p.Execute(conn, file, fileInfo.Size, bar)
	if err != nil {
		return 0, fmt.Errorf("error sending file data: %v", err)
	}

	return sentBytes, nil
}

func writeFileHeader(conn io.Writer, fileInfo types.FileInfo) error {
	header := fmt.Sprintf("FILE:%s:%d\n", fileInfo.Name, fileInfo.Size)
	if _, err := conn.Write([]byte(header)); err != nil {
		return fmt.Errorf("error sending file header: %v", err)
	}
	return nil
}
