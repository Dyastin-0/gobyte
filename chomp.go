package main

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
			fmt.Println(INFO.Render(fmt.Sprintf("file chomping request from %s (%s)", msg.SenderName, msg.IPAddress)))

			str := "file"
			if len(msg.Files) > 1 {
				str += "s"
			}
			confirm, err := c.showConfirm(fmt.Sprintf("accept %d %s from %s?", len(msg.Files), str, msg.SenderName), 15*time.Second)
			if err != nil {
				fmt.Println(ERROR.Render(fmt.Sprintf("request from %s timed out", msg.SenderName)))
				continue
			}

			err = c.sendAck(msg, confirm)
			if err != nil {
				fmt.Println(ERROR.Render(fmt.Sprintf("error: %v", err)))
			}

			if !confirm {
				fmt.Println(INFO.Render("files rejected"))
				continue
			}

			listener.(*net.TCPListener).SetDeadline(time.Now().Add(15 * time.Second))
			fmt.Println(INFO.Render("waiting for connection..."))

			go func() {
				err := chomp(listener, dir)
				if err != nil {
					fmt.Println(ERROR.Render(fmt.Sprintf("error chomping: %v", err)))
					return
				}

				fmt.Println(SUCCESS.Bold(true).Render("all files chomped ✓"))
			}()
		}
	}
}

func (c *Client) sendAck(msg Message, confirm bool) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", msg.IPAddress, discoveryPort))
	if err != nil {
		return fmt.Errorf("failed to resolve udp addr")
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to dial udp")
	}

	defer conn.Close()

	ack := Message{
		Type:       TypeTransferAck,
		SenderID:   c.self.ID,
		SenderName: c.self.Name,
		IPAddress:  c.self.IPAddress,
		Accepted:   confirm,
		TransferID: msg.TransferID,
	}

	ackBytes, err := json.Marshal(ack)
	if err != nil {
		return fmt.Errorf("failed to marshal ackMessage")
	}

	_, err = conn.Write(ackBytes)
	if err != nil {
		return fmt.Errorf("failed to write ackMessage")
	}

	return nil
}

func chomp(listener net.Listener, dir string) error {
	conn, err := listener.Accept()
	if err != nil {
		return fmt.Errorf("failed to accept tcp connection")
	}
	defer conn.Close()

	fmt.Println(INFO.Render("Connected. Receiving files..."))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s", dir)
	}

	reader := bufio.NewReader(conn)

	for {
		header, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading file header")
		}

		header = strings.TrimSpace(header)
		if header == "END" {
			break
		}

		if !strings.HasPrefix(header, "FILE:") {
			return fmt.Errorf("invalid file header: %s", header)
		}

		parts := strings.Split(header, ":")
		if len(parts) != 3 {
			return fmt.Errorf("invalid file header: %s", header)
		}

		fileName := parts[1]

		fileSize, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid file size: %d", fileSize)
		}

		wroteBytes, err := writeBytes(reader, fileSize, dir, fileName)
		if err != nil {
			return err
		}

		fmt.Println(SUCCESS.Render(fmt.Sprintf("received %s (%d bytes)", fileName, wroteBytes)))
	}

	return nil
}

func writeBytes(reader io.Reader, fileSize int64, dir, fileName string) (int64, error) {
	filePath := filepath.Join(dir, fileName)

	if _, err := os.Stat(filePath); err == nil {
		base := filepath.Base(fileName)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		filePath = filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, time.Now().Unix(), ext))
	}

	file, err := os.Create(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to create file: %s", filePath)
	}

	defer file.Close()

	copiedBytes, err := io.CopyN(file, reader, fileSize)
	if err != nil {
		file.Close()
		return 0, fmt.Errorf("failed to copy file: %s", file.Name())
	}

	return copiedBytes, nil
}
