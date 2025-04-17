package client

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

	"github.com/Dyastin-0/gobyte/styles"
	"github.com/Dyastin-0/gobyte/types"
)

func (c *Client) StartChompListener(ctx context.Context, dir string, onRequest func(msg types.Message) (bool, error)) {
	go c.presenceBroadcaster(ctx)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", c.transferPort))
	if err != nil {
		fmt.Println(styles.ERROR.Render(fmt.Sprintf("failed to create tcp listener: %v", err)))
		return
	}

	for {
		select {
		case <-ctx.Done():
			listener.Close()
			return
		case msg := <-c.transferReqChan:
			fmt.Println(styles.INFO.Render(fmt.Sprintf("chuck request from %s (%s)", msg.SenderName, msg.IPAddress)))

			confirm, err := onRequest(msg)
			if err != nil {
				fmt.Println(styles.ERROR.Render(fmt.Sprintf("request from %s timed out", msg.SenderName)))
				continue
			}

			err = c.sendAck(msg, confirm)
			if err != nil {
				fmt.Println(styles.ERROR.Render(fmt.Sprintf("%v", err)))
				continue
			}

			if !confirm {
				fmt.Println(styles.INFO.Render("files rejected"))
				continue
			}

			listener.(*net.TCPListener).SetDeadline(time.Now().Add(15 * time.Second))

			go func() {
				if err := c.readFiles(listener, dir); err != nil {
					fmt.Println(styles.ERROR.Render(fmt.Sprintf("failed to chomp: %v", err)))
					return
				}

				fmt.Println(styles.SUCCESS.Bold(true).Render("all files chomped ✓"))
			}()
		}
	}
}

func (c *Client) sendAck(msg types.Message, confirm bool) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", msg.IPAddress, c.discoveryPort))
	if err != nil {
		return fmt.Errorf("failed to resolve udp addr: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to dial udp: %v", err)
	}

	defer conn.Close()

	ack := types.Message{
		Type:       types.TypeTransferAck,
		SenderID:   c.Self.ID,
		SenderName: c.Self.Name,
		IPAddress:  c.Self.IPAddress,
		Accepted:   confirm,
		TransferID: msg.TransferID,
	}

	ackBytes, err := json.Marshal(ack)
	if err != nil {
		return fmt.Errorf("failed to marshal ackMessage: %v", err)
	}

	_, err = conn.Write(ackBytes)
	if err != nil {
		return fmt.Errorf("failed to write ackMessage: %v", err)
	}

	return nil
}

func (c *Client) readFiles(listener net.Listener, dir string) error {
	conn, err := listener.Accept()
	if err != nil {
		return fmt.Errorf("failed to accept tcp connection: %v", err)
	}

	defer conn.Close()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	reader := bufio.NewReader(conn)

	for {
		header, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}

			fmt.Println(styles.ERROR.Render(fmt.Sprintf("error reading file header: %v", err)))
			continue
		}

		header = strings.TrimSpace(header)
		if header == "END" {
			break
		}

		fileName, fileSize, err := readFileHeader(header)
		if err != nil {
			fmt.Println(styles.ERROR.Render(err.Error()))
			continue
		}

		wroteBytes, err := writeBytesToDir(reader, fileSize, dir, fileName)
		if err != nil {
			fmt.Println(styles.ERROR.Render(err.Error()))
			continue
		}

		fmt.Println(styles.SUCCESS.Render(fmt.Sprintf("chomped %s (%d bytes)", fileName, wroteBytes)))
	}

	return nil
}

func writeBytesToDir(reader io.Reader, fileSize int64, dir, fileName string) (int64, error) {
	filePath := filepath.Join(dir, fileName)

	if _, err := os.Stat(filePath); err == nil {
		base := filepath.Base(fileName)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		filePath = filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, time.Now().Unix(), ext))
	}

	file, err := os.Create(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to create %s: %v", filePath, err)
	}

	defer file.Close()

	copiedBytes, err := io.CopyN(file, reader, fileSize)
	if err != nil {
		return 0, fmt.Errorf("failed to copy %s: %v", file.Name(), err)
	}

	return copiedBytes, nil
}

func readFileHeader(header string) (string, int64, error) {
	parts := strings.Split(header, ":")
	if len(parts) != 3 {
		return "", 0, fmt.Errorf("invalid file header: %s", header)
	}

	if !strings.HasPrefix(header, "FILE:") {
		return "", 0, fmt.Errorf("invalid header: %s", header)
	}

	fileName := parts[1]

	fileSize, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("%s has invalid size: %d", fileName, fileSize)
	}

	return fileName, fileSize, nil
}
