package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v3"
)

const (
	discoveryPort = 8888
	transferPort  = 8889
	broadcastAddr = "255.255.255.255"
	discoveryMsg  = "LANSHARE_DISCOVERY"
	maxBufferSize = 1024 * 1024 // 1MB chunks for file transfer
)

// Peer represents another user on the LAN
type Peer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
}

// FileInfo represents a file to be shared
type FileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Path string `json:"path"`
}

// Message types for UDP communication
type MessageType string

const (
	TypeDiscovery    MessageType = "discovery"
	TypeDiscoveryAck MessageType = "discovery_ack"
	TypeTransferReq  MessageType = "transfer_req"
	TypeTransferAck  MessageType = "transfer_ack"
)

// Message represents the structure of UDP messages
type Message struct {
	Type       MessageType `json:"type"`
	SenderID   string      `json:"sender_id"`
	SenderName string      `json:"sender_name"`
	IPAddress  string      `json:"ip_address"`
	Files      []FileInfo  `json:"files,omitempty"`
	Peers      []string    `json:"peers,omitempty"` // IDs of peers to send to
}

// Global variables
var (
	localPeer                     Peer
	knownPeers                    = make(map[string]Peer)
	peersMutex                    sync.RWMutex
	selectedFiles                 []FileInfo
	discoveryCtx, discoveryCancel = context.WithCancel(context.Background())
)

// Styling
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	infoStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("#888888"))
)

func main() {
	// Initialize local peer info
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}

	localPeer = Peer{
		ID:        fmt.Sprintf("%s-%d", hostname, time.Now().Unix()),
		Name:      hostname,
		IPAddress: getLocalIP(),
	}

	// Create CLI app
	app := &cli.Command{
		Name:        "lanshare",
		Usage:       "Share files on your local network",
		Version:     "1.0.0",
		Description: "A command-line tool for sharing files with peers on your local network",
		Commands: []*cli.Command{
			{
				Name:    "start",
				Aliases: []string{"s"},
				Usage:   "Start LAN Share in interactive mode",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "name",
						Aliases: []string{"n"},
						Usage:   "Override the default device name",
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.String("name") != "" {
						localPeer.Name = c.String("name")
					}
					fmt.Println(titleStyle.Render(" LAN Share "))
					fmt.Printf("Running as: %s (%s)\n", localPeer.Name, localPeer.IPAddress)
					runInteractiveMode()
					return nil
				},
			},
			{
				Name:    "list",
				Aliases: []string{"ls", "l"},
				Usage:   "List available peers on the network",
				Action: func(ctx context.Context, c *cli.Command) error {
					fmt.Println("Discovering peers...")
					discoverPeers(5) // 5 seconds timeout
					displayPeers()
					return nil
				},
			},
			{
				Name:    "send",
				Aliases: []string{"s"},
				Usage:   "Send files to peers",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:     "file",
						Aliases:  []string{"f"},
						Usage:    "File(s) to send (repeat flag for multiple files)",
						Required: true,
					},
					&cli.StringSliceFlag{
						Name:    "peer",
						Aliases: []string{"p"},
						Usage:   "Peer ID(s) to send to (repeat flag for multiple peers)",
					},
					&cli.BoolFlag{
						Name:    "interactive",
						Aliases: []string{"i"},
						Usage:   "Use interactive mode to select peers",
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					files, err := resolveFiles(c.StringSlice("file"))
					if err != nil {
						return err
					}

					if c.Bool("interactive") {
						discoverPeers(3)
						peers, err := selectPeersToSendTo()
						if err != nil || len(peers) == 0 {
							return fmt.Errorf("no peers selected")
						}

						for _, peer := range peers {
							sendFilesToPeer(files, peer)
						}
					} else if len(c.StringSlice("peer")) > 0 {
						discoverPeers(3)

						for _, peerID := range c.StringSlice("peer") {
							peersMutex.RLock()
							peer, exists := knownPeers[peerID]
							peersMutex.RUnlock()

							if !exists {
								fmt.Printf("Peer ID %s not found\n", peerID)
								continue
							}

							sendFilesToPeer(files, peer)
						}
					} else {
						return fmt.Errorf("must specify peers with --peer or use --interactive")
					}

					return nil
				},
			},
			{
				Name:    "listen",
				Aliases: []string{"l"},
				Usage:   "Listen for incoming files",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "dir",
						Aliases: []string{"d"},
						Usage:   "Directory to save received files",
						Value:   "./downloads",
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					downloadDir := c.String("dir")
					if err := os.MkdirAll(downloadDir, 0755); err != nil {
						return err
					}

					fmt.Printf("Listening for incoming files. Files will be saved to %s\n", downloadDir)
					startDiscovery()

					// Keep the application running
					<-ctx.Done()
					return nil
				},
			},
		},
	}

	// Run the app
	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func runInteractiveMode() {
	fmt.Println(infoStyle.Render("Discovering peers on your network..."))

	// Start discovery in background
	go startDiscovery()

	// Wait a moment for initial discovery
	time.Sleep(2 * time.Second)

	// Main menu loop
	for {
		option := showMainMenu()
		switch option {
		case "send":
			handleSendFiles()
		case "refresh":
			refreshPeers()
		case "quit":
			discoveryCancel()
			fmt.Println("Goodbye!")
			return
		}
	}
}

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

func discoverPeers(timeoutSeconds int) {
	// Start discovery in the background
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Listen for incoming discovery messages
	go listenForDiscoveryWithTimeout(ctx)

	// Broadcast our presence
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

	// Send discovery broadcast
	sendDiscoveryBroadcast(conn)

	// Wait for the timeout to expire
	<-ctx.Done()
}

func listenForDiscoveryWithTimeout(ctx context.Context) {
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

	// Set read deadline to enable context cancellation
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	buffer := make([]byte, 1024)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Reset the read deadline
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))

			n, remoteAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// This is just a timeout, continue the loop
					continue
				}
				fmt.Printf("Error reading from UDP: %v\n", err)
				continue
			}

			// Process the incoming message
			var msg Message
			if err := json.Unmarshal(buffer[:n], &msg); err != nil {
				fmt.Printf("Error unmarshaling message: %v\n", err)
				continue
			}

			// Skip messages from ourselves
			if msg.SenderID == localPeer.ID {
				continue
			}

			// Handle the message
			switch msg.Type {
			case TypeDiscovery:
				handleDiscoveryMessage(msg, remoteAddr, conn)
			case TypeDiscoveryAck:
				handleDiscoveryAck(msg)
			}
		}
	}
}

func displayPeers() {
	peersMutex.RLock()
	defer peersMutex.RUnlock()

	if len(knownPeers) == 0 {
		fmt.Println("No peers found on the network.")
		return
	}

	fmt.Printf("Found %d peers on the network:\n", len(knownPeers))
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("%-20s %-15s %s\n", "NAME", "IP ADDRESS", "ID")
	fmt.Println(strings.Repeat("-", 50))

	for _, peer := range knownPeers {
		fmt.Printf("%-20s %-15s %s\n", peer.Name, peer.IPAddress, peer.ID)
	}
}

func startDiscovery() {
	// Listen for incoming discovery messages
	go listenForDiscovery()

	// Periodically broadcast our presence
	go broadcastPresence()
}

func broadcastPresence() {
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

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Send initial broadcast
	sendDiscoveryBroadcast(conn)

	for {
		select {
		case <-ticker.C:
			sendDiscoveryBroadcast(conn)
		case <-discoveryCtx.Done():
			return
		}
	}
}

func sendDiscoveryBroadcast(conn *net.UDPConn) {
	msg := Message{
		Type:       TypeDiscovery,
		SenderID:   localPeer.ID,
		SenderName: localPeer.Name,
		IPAddress:  localPeer.IPAddress,
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

func listenForDiscovery() {
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

	// Set read deadline to enable context cancellation
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	buffer := make([]byte, 1024)

	for {
		select {
		case <-discoveryCtx.Done():
			return
		default:
			// Reset the read deadline
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))

			n, remoteAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// This is just a timeout, continue the loop
					continue
				}
				fmt.Printf("Error reading from UDP: %v\n", err)
				continue
			}

			// Process the incoming message
			var msg Message
			if err := json.Unmarshal(buffer[:n], &msg); err != nil {
				fmt.Printf("Error unmarshaling message: %v\n", err)
				continue
			}

			// Skip messages from ourselves
			if msg.SenderID == localPeer.ID {
				continue
			}

			// Handle the message
			switch msg.Type {
			case TypeDiscovery:
				handleDiscoveryMessage(msg, remoteAddr, conn)
			case TypeDiscoveryAck:
				handleDiscoveryAck(msg)
			case TypeTransferReq:
				handleTransferRequest(msg)
			}
		}
	}
}

func handleDiscoveryMessage(msg Message, remoteAddr *net.UDPAddr, conn *net.UDPConn) {
	// Add the peer to our known peers
	peer := Peer{
		ID:        msg.SenderID,
		Name:      msg.SenderName,
		IPAddress: msg.IPAddress,
	}

	peersMutex.Lock()
	knownPeers[peer.ID] = peer
	peersMutex.Unlock()

	// Send an acknowledgment
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
	// Set up a TCP server to receive files
	go func() {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", transferPort))
		if err != nil {
			fmt.Printf("Error setting up file receiver: %v\n", err)
			return
		}
		defer listener.Close()

		fmt.Printf("\nIncoming file transfer from %s...\n", msg.SenderName)

		// Accept the connection
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Error accepting connection: %v\n", err)
			return
		}
		defer conn.Close()

		// Create downloads directory if it doesn't exist
		downloadDir := filepath.Join(".", "downloads")
		if err := os.MkdirAll(downloadDir, 0755); err != nil {
			fmt.Printf("Error creating downloads directory: %v\n", err)
			return
		}

		// Receive files
		for _, fileInfo := range msg.Files {
			fmt.Printf("Receiving %s (%d bytes)...\n", fileInfo.Name, fileInfo.Size)

			// Create the file
			filePath := filepath.Join(downloadDir, fileInfo.Name)
			file, err := os.Create(filePath)
			if err != nil {
				fmt.Printf("Error creating file: %v\n", err)
				continue
			}

			// Receive the file data
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
				fmt.Printf("\rProgress: %d%%", int(float64(received)/float64(fileInfo.Size)*100))
			}

			file.Close()
			fmt.Printf("\nSaved to %s\n", filePath)
		}

		fmt.Println("File transfer complete")
		// Prompt to continue
		fmt.Println("\nPress Enter to continue...")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
	}()
}

func handleSendFiles() {
	// First, select files to send
	files, err := selectFilesToSend()
	if err != nil || len(files) == 0 {
		fmt.Println("No files selected.")
		return
	}

	// Then, select peers to send to
	peers, err := selectPeersToSendTo()
	if err != nil || len(peers) == 0 {
		fmt.Println("No peers selected.")
		return
	}

	// Send files to selected peers
	for _, peer := range peers {
		sendFilesToPeer(files, peer)
	}
}

func selectFilesToSend() ([]FileInfo, error) {
	// Get files in current directory
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}

	var fileOptions []huh.Option[string]
	for _, entry := range entries {
		if !entry.IsDir() {
			fileInfo, err := entry.Info()
			if err != nil {
				continue
			}

			option := fmt.Sprintf("%s (%s)", entry.Name(), formatSize(fileInfo.Size()))
			fileOptions = append(fileOptions, huh.NewOption(option, entry.Name()))
		}
	}

	// Allow selecting multiple files
	var selectedFileNames []string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select files to share").
				Options(fileOptions...).
				Value(&selectedFileNames),
		),
	)

	err = form.Run()
	if err != nil {
		return nil, err
	}

	// Convert selected file names to FileInfo
	var selectedFiles []FileInfo
	for _, name := range selectedFileNames {
		fileInfo, err := os.Stat(name)
		if err != nil {
			continue
		}

		selectedFiles = append(selectedFiles, FileInfo{
			Name: name,
			Size: fileInfo.Size(),
			Path: filepath.Join(".", name),
		})
	}

	return selectedFiles, nil
}

func selectPeersToSendTo() ([]Peer, error) {
	peersMutex.RLock()
	defer peersMutex.RUnlock()

	if len(knownPeers) == 0 {
		fmt.Println("No peers found on the network.")
		return nil, fmt.Errorf("no peers found")
	}

	var peerOptions []huh.Option[string]
	peerMap := make(map[string]Peer)

	for _, peer := range knownPeers {
		option := fmt.Sprintf("%s (%s)", peer.Name, peer.IPAddress)
		peerOptions = append(peerOptions, huh.NewOption(option, peer.ID))
		peerMap[peer.ID] = peer
	}

	// Allow selecting multiple peers
	var selectedPeerIDs []string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select peers to send to").
				Options(peerOptions...).
				Value(&selectedPeerIDs),
		),
	)

	err := form.Run()
	if err != nil {
		return nil, err
	}

	// Convert selected peer IDs to Peer objects
	var selectedPeers []Peer
	for _, id := range selectedPeerIDs {
		selectedPeers = append(selectedPeers, peerMap[id])
	}

	return selectedPeers, nil
}

func sendFilesToPeer(files []FileInfo, peer Peer) {
	// First, send a transfer request via UDP
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

	// Create and send transfer request
	msg := Message{
		Type:       TypeTransferReq,
		SenderID:   localPeer.ID,
		SenderName: localPeer.Name,
		IPAddress:  localPeer.IPAddress,
		Files:      files,
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Error marshaling transfer request: %v\n", err)
		conn.Close()
		return
	}

	_, err = conn.Write(jsonData)
	if err != nil {
		fmt.Printf("Error sending transfer request: %v\n", err)
		conn.Close()
		return
	}

	conn.Close()

	// Give the receiver a moment to set up
	time.Sleep(1 * time.Second)

	// Connect to the peer via TCP to transfer files
	tcpConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", peer.IPAddress, transferPort))
	if err != nil {
		fmt.Printf("Error connecting to peer for file transfer: %v\n", err)
		return
	}
	defer tcpConn.Close()

	// Send each file
	for _, fileInfo := range files {
		fmt.Printf("Sending %s to %s...\n", fileInfo.Name, peer.Name)

		file, err := os.Open(fileInfo.Path)
		if err != nil {
			fmt.Printf("Error opening file: %v\n", err)
			continue
		}

		buffer := make([]byte, maxBufferSize)
		sent := int64(0)

		for sent < fileInfo.Size {
			n, err := file.Read(buffer)
			if err != nil && err != io.EOF {
				fmt.Printf("Error reading file: %v\n", err)
				break
			}

			if n == 0 {
				break
			}

			_, err = tcpConn.Write(buffer[:n])
			if err != nil {
				fmt.Printf("Error sending file data: %v\n", err)
				break
			}

			sent += int64(n)
			fmt.Printf("\rProgress: %d%%", int(float64(sent)/float64(fileInfo.Size)*100))
		}

		file.Close()
		fmt.Println("\nFile sent successfully")
	}

	fmt.Printf("All files sent to %s\n", peer.Name)
}

func refreshPeers() {
	fmt.Println("Refreshing peer list...")

	// Clear current peers
	peersMutex.Lock()
	knownPeers = make(map[string]Peer)
	peersMutex.Unlock()

	// Send a discovery broadcast
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

	sendDiscoveryBroadcast(conn)

	// Wait a moment for responses
	fmt.Println("Waiting for peers to respond...")
	time.Sleep(2 * time.Second)

	// Show the updated peer list
	peersMutex.RLock()
	fmt.Printf("Found %d peers on the network\n", len(knownPeers))
	peersMutex.RUnlock()
}

func showMainMenu() string {
	var option string

	// Count peers
	peersMutex.RLock()
	peerCount := len(knownPeers)
	peersMutex.RUnlock()

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(
					huh.NewOption(fmt.Sprintf("Send Files (%d peers available)", peerCount), "send"),
					huh.NewOption("Refresh Peer List", "refresh"),
					huh.NewOption("Quit", "quit"),
				).
				Value(&option),
		),
	)

	err := form.Run()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return "quit"
	}

	return option
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
