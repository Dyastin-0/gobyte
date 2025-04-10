package gobyte

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
)

func resolveFiles(filePaths []string) ([]FileInfo, error) {
	var files []FileInfo

	for _, path := range filePaths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("error accessing file %s: %v", path, err)
		}

		if fileInfo.IsDir() {
			return nil, fmt.Errorf("%s is a directory, not a file", path)
		}

		files = append(files, FileInfo{
			Name: filepath.Base(path),
			Size: fileInfo.Size(),
			Path: path,
		})
	}

	return files, nil
}

func handleDiscoveryMessage(msg Message, remoteAddr *net.UDPAddr, conn *net.UDPConn) {
	peer := Peer{
		ID:        msg.SenderID,
		Name:      msg.SenderName,
		IPAddress: msg.IPAddress,
	}

	peersMutex.Lock()
	knownPeers[peer.ID] = peer
	peersMutex.Unlock()

	ackMsg := Message{
		Type:       TypeDiscoveryAck,
		SenderID:   localPeer.ID,
		SenderName: localPeer.Name,
		IPAddress:  localPeer.IPAddress,
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

func handleDiscoveryAck(msg Message) {
	peer := Peer{
		ID:        msg.SenderID,
		Name:      msg.SenderName,
		IPAddress: msg.IPAddress,
	}

	peersMutex.Lock()
	knownPeers[peer.ID] = peer
	peersMutex.Unlock()
}

func handleTransferRequest(msg Message) {
	go func() {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", transferPort))
		if err != nil {
			fmt.Printf("Error setting up file receiver: %v\n", err)
			return
		}
		defer listener.Close()

		fmt.Printf("\nIncoming file transfer from %s...\n", msg.SenderName)

		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Error accepting connection: %v\n", err)
			return
		}
		defer conn.Close()

		downloadDir := filepath.Join(".", "files")
		if err := os.MkdirAll(downloadDir, 0755); err != nil {
			fmt.Printf("Error creating downloads directory: %v\n", err)
			return
		}

		for _, fileInfo := range msg.Files {
			fmt.Printf("Receiving %s (%d bytes)...\n", fileInfo.Name, fileInfo.Size)

			filePath := filepath.Join(downloadDir, fileInfo.Name)
			file, err := os.Create(filePath)
			if err != nil {
				fmt.Printf("Error creating file: %v\n", err)
				continue
			}

			received := int64(0)
			buffer := make([]byte, maxBufferSize)

			for received < fileInfo.Size {
				n, err := conn.Read(buffer)
				if err != nil && err != io.EOF {
					fmt.Printf("Error receiving file data: %v\n", err)
					break
				}

				if n == 0 {
					break
				}

				_, err = file.Write(buffer[:n])
				if err != nil {
					fmt.Printf("Error writing to file: %v\n", err)
					break
				}

				received += int64(n)
				fmt.Printf("Progress: %d%%", int(float64(received)/float64(fileInfo.Size)*100))
			}

			file.Close()
			fmt.Printf("Saved to %s\n", filePath)
		}

		fmt.Println("File transfer complete")
	}()
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}

	return "127.0.0.1"
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
