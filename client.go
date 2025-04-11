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
	"sync"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/google/uuid"
	"github.com/urfave/cli/v3"
)

type Client struct {
	Self             *Peer
	Hostname         string
	KnownPeers       map[string]*Peer
	SelectedFiles    []FileInfo
	MU               sync.RWMutex
	transferReqChan  chan Message
	PendingTransfers map[string]chan bool
}

func NewClient(ctx context.Context) Client {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}
	return Client{
		Self: &Peer{
			ID:        fmt.Sprintf("%s-%s", hostname, uuid.New()),
			Name:      hostname,
			IPAddress: getLocalIP(),
		},
		KnownPeers:       make(map[string]*Peer),
		SelectedFiles:    make([]FileInfo, 0),
		transferReqChan:  make(chan Message, 10),
		PendingTransfers: make(map[string]chan bool),
	}
}

func (c *Client) Run(ctx context.Context, cancel context.CancelFunc) {
	app := &cli.Command{
		Name:        "gobyte",
		Usage:       "Share files on your local network",
		Version:     "1.0.0",
		Description: "A command-line tool for sharing files with peers on your local network",
		Commands: []*cli.Command{
			{
				Name:    "chuck",
				Aliases: []string{"ck"},
				Usage:   "Send files to discovered peers",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "name",
						Aliases: []string{"n"},
						Usage:   "Override the default device name",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if cmd.String("name") != "" {
						c.Self.Name = cmd.String("name")
					}
					fmt.Println(TITLE.Render("GOBYTE"))
					fmt.Printf("Running as: %s (%s)\n", c.Self.Name, c.Self.IPAddress)
					c.runInteractiveMode(ctx, cancel)
					return nil
				},
			},
			{
				Name:    "list",
				Aliases: []string{"ls", "l"},
				Usage:   "List available peers on the network",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					fmt.Println("Discovering peers...")
					c.discoverPeers(5)
					c.displayPeers()
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
				Action: func(ctx context.Context, cmd *cli.Command) error {
					files, err := resolveFiles(cmd.StringSlice("file"))
					if err != nil {
						return err
					}
					if cmd.Bool("interactive") {
						c.discoverPeers(3)
						peers, err := c.selectPeers()
						if err != nil || len(peers) == 0 {
							return fmt.Errorf("no peers selected")
						}
						for _, peer := range peers {
							c.sendFilesTo(&peer, files)
						}
					} else if len(cmd.StringSlice("peer")) > 0 {
						c.discoverPeers(3)
						for _, peerID := range cmd.StringSlice("peer") {
							c.MU.RLock()
							peer, exists := c.KnownPeers[peerID]
							c.MU.RUnlock()
							if !exists {
								fmt.Printf("Peer ID %s not found\n", peerID)
								continue
							}
							c.sendFilesTo(peer, files)
						}
					} else {
						return fmt.Errorf("must specify peers with --peer or use --interactive")
					}
					return nil
				},
			},
			{
				Name:    "chomp",
				Aliases: []string{"l"},
				Usage:   "Listen for incoming files",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "dir",
						Aliases: []string{"d"},
						Usage:   "Directory to save received files",
						Value:   "./files",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					downloadDir := cmd.String("dir")
					if err := os.MkdirAll(downloadDir, 0755); err != nil {
						return err
					}
					fmt.Println(INFO.Render(fmt.Sprintf("Listening for incoming files. Files will be saved to %s", downloadDir)))

					go c.handleTransferRequests(ctx, downloadDir)

					go c.listen(ctx)
					go c.broadcastPresence(ctx)

					<-ctx.Done()
					return nil
				},
			},
		},
	}
	if err := app.Run(ctx, os.Args); err != nil {
		panic(err)
	}
}

func (c *Client) sendFilesTo(peer *Peer, files []FileInfo) {
	// Generate a unique transfer ID
	transferID := uuid.New().String()

	// Create UDP connection for sending the request
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

	// Create a channel to receive the acceptance response
	ackChan := make(chan bool)

	// Register this transfer in a map of pending transfers
	c.MU.Lock()
	if c.PendingTransfers == nil {
		c.PendingTransfers = make(map[string]chan bool)
	}
	c.PendingTransfers[transferID] = ackChan
	c.MU.Unlock()

	// Clean up when done
	defer func() {
		conn.Close()
		c.MU.Lock()
		delete(c.PendingTransfers, transferID)
		c.MU.Unlock()
	}()

	// Send the transfer request
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

	fmt.Printf("Transfer request sent to %s. Waiting for acceptance...\n", peer.Name)

	// Wait for acceptance with a timeout
	select {
	case accepted := <-ackChan:
		if !accepted {
			fmt.Printf("%s rejected the file transfer.\n", peer.Name)
			return
		}
		fmt.Printf("%s accepted the file transfer. Sending files...\n", peer.Name)

		// Proceed with file transfer
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
			fmt.Printf("Sending %s to %s...\n", fileInfo.Name, peer.Name)
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
			fmt.Printf("\nSent %s (%d bytes)\n", fileInfo.Name, sent)
		}

		writer.WriteString("END\n")
		writer.Flush()
		fmt.Printf("All files sent to %s\n", peer.Name)

	case <-time.After(30 * time.Second):
		fmt.Printf("Timeout waiting for %s to accept the transfer.\n", peer.Name)
		return
	}
}

func (c *Client) selectPeers() ([]Peer, error) {
	c.MU.RLock()
	defer c.MU.RUnlock()

	if len(c.KnownPeers) == 0 {
		return nil, fmt.Errorf("no peers found")
	}

	var peerOptions []huh.Option[string]
	peerMap := make(map[string]Peer)

	for _, peer := range c.KnownPeers {
		option := fmt.Sprintf("%s (%s)", peer.Name, peer.IPAddress)
		peerOptions = append(peerOptions, huh.NewOption(option, peer.ID))
		peerMap[peer.ID] = *peer
	}

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

	var selectedPeers []Peer
	for _, id := range selectedPeerIDs {
		selectedPeers = append(selectedPeers, peerMap[id])
	}

	return selectedPeers, nil
}

func (c *Client) sendFiles() {
	peers, err := c.selectPeers()
	if err != nil || len(peers) == 0 {
		fmt.Println(INFO.Render("No peers to send to."))
		return
	}

	files, err := c.selectFiles()
	if err != nil || len(files) == 0 {
		fmt.Println(INFO.Render(err.Error()))
		return
	}

	for _, peer := range peers {
		c.sendFilesTo(&peer, files)
	}
}

func (c *Client) selectFiles() ([]FileInfo, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}

	var fileOptions []huh.Option[string]
	for _, entry := range entries {
		if !entry.IsDir() {
			fileInfo, enterr := entry.Info()
			if enterr != nil {
				continue
			}
			option := fmt.Sprintf("%s (%d)", entry.Name(), fileInfo.Size())
			fileOptions = append(fileOptions, huh.NewOption(option, entry.Name()))
		}
	}

	if len(fileOptions) == 0 {
		return nil, fmt.Errorf("no files found in current directory")
	}

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

	if len(selectedFileNames) == 0 {
		return nil, fmt.Errorf("no files selected")
	}

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

func (c *Client) handleTransferRequests(ctx context.Context, downloadDir string) {
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
			fmt.Println(INFO.Render(fmt.Sprintf("\nFile chomping request from %s", msg.SenderName)))
			confirm := c.showConfirm(fmt.Sprintf("Accept %d files from %s?", len(msg.Files), msg.SenderName))

			// Send acceptance response back
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

			// Create and send the response
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

			// If accepted, prepare to receive files
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

				// Rest of the file receiving logic remains the same
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
					fmt.Println(INFO.Render(fmt.Sprintf("Received %s (%d bytes)\n", fileName, received)))
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
				c.handleDiscoveryMessage(msg, remoteAddr, conn)
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

func (c *Client) refreshPeers() {
	fmt.Println(INFO.Render("Refreshing peer list..."))

	c.MU.Lock()
	c.KnownPeers = make(map[string]*Peer)
	c.MU.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	c.broadcastDiscovery()

	<-ctx.Done()

	c.MU.RLock()
	peerCount := len(c.KnownPeers)
	c.MU.RUnlock()

	if peerCount > 0 {
		fmt.Printf(INFO.Render("Found %d peers on the network.\n"), peerCount)
	} else {
		fmt.Println(INFO.Render("No peers found."))
	}
}

func (c *Client) broadcastDiscovery() {
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

	ticker := time.NewTicker(10 * time.Second)
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

func (c *Client) runInteractiveMode(ctx context.Context, cancel context.CancelFunc) {
	fmt.Println(INFO.Render("Discovering peers on your network..."))

	go c.listen(ctx)

	for {
		option := c.showMainMenu()

		switch option {
		case "send":
			c.sendFiles()

		case "refresh":
			c.refreshPeers()

		case "quit":
			cancel()
			fmt.Println("Goodbye!")
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

	for i := 0; i < 3; i++ {
		_, err = conn.Write(jsonData)
		if err != nil {
			fmt.Printf("Error sending discovery broadcast: %v\n", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (c *Client) discoverPeers(timeoutSeconds int) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	c.MU.Lock()
	c.KnownPeers = make(map[string]*Peer)
	c.MU.Unlock()

	go func() {
		for i := 0; i < 3; i++ {
			c.broadcastDiscovery()
			time.Sleep(500 * time.Millisecond)
		}
	}()

	<-ctx.Done()
}

func (c *Client) displayPeers() {
	c.MU.RLock()
	defer c.MU.RUnlock()

	if len(c.KnownPeers) == 0 {
		fmt.Println(INFO.Render("No peers found on the network."))
		return
	}

	fmt.Printf(INFO.Render("Found %d peers on the network:\n"), len(c.KnownPeers))
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("%-20s %-15s %s\n", "NAME", "IP ADDRESS", "ID")
	fmt.Println(strings.Repeat("-", 50))

	for _, peer := range c.KnownPeers {
		fmt.Printf("%-20s %-15s %s\n", peer.Name, peer.IPAddress, peer.ID)
	}
}

func (c *Client) handleDiscoveryMessage(msg Message, remoteAddr *net.UDPAddr, conn *net.UDPConn) {
	peer := Peer{
		ID:        msg.SenderID,
		Name:      msg.SenderName,
		IPAddress: msg.IPAddress,
	}

	c.MU.Lock()
	c.KnownPeers[peer.ID] = &peer
	c.MU.Unlock()

	ackMsg := Message{
		Type:       TypeDiscoveryAck,
		SenderID:   c.Self.ID,
		SenderName: c.Self.Name,
		IPAddress:  c.Self.IPAddress,
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

func (c *Client) handleDiscoveryAck(msg Message) {
	peer := Peer{
		ID:        msg.SenderID,
		Name:      msg.SenderName,
		IPAddress: msg.IPAddress,
	}

	c.MU.Lock()
	c.KnownPeers[peer.ID] = &peer
	c.MU.Unlock()
}

func (c *Client) showConfirm(title string) bool {
	var confirm bool

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Affirmative("Yes").
				Negative("No").
				Title(title).
				Value(&confirm),
		),
	)

	err := form.Run()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return false
	}

	return confirm
}

func (c *Client) showMainMenu() string {
	var option string

	c.MU.RLock()
	peerCount := len(c.KnownPeers)
	c.MU.RUnlock()

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
