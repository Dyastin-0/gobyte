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
			confirm, err := c.showConfirm(fmt.Sprintf("Accept %d %s from %s?", len(msg.Files), str, msg.SenderName), 15*time.Second)
			if err != nil {
				fmt.Println(ERROR.Render("request timeout."))
				continue
			}

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

			listener.(*net.TCPListener).SetDeadline(time.Now().Add(15 * time.Millisecond))
			fmt.Println(INFO.Render("Waiting for connection..."))

			go chomp(listener, dir)
		}
	}
}

func chomp(listener net.Listener, dir string) {
	conn, err := listener.Accept()
	if err != nil {
		fmt.Printf("Error accepting connection: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println(INFO.Render("Connected. Receiving files..."))
	if err := os.MkdirAll(dir, 0755); err != nil {
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

		filePath := filepath.Join(dir, fileName)

		if _, err = os.Stat(filePath); err == nil {
			base := filepath.Base(fileName)
			ext := filepath.Ext(base)
			name := strings.TrimSuffix(base, ext)
			filePath = filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, time.Now().Unix(), ext))
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

	fmt.Println(SUCCESS.Bold(true).Render("File chomping complete ✓"))
}
