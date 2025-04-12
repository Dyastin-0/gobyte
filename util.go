package gobyte

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

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

	fmt.Println(SUCCESS.Render("File chomping complete ✓"))
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
