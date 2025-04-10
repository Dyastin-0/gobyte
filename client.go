package gobyte

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/google/uuid"
	"github.com/urfave/cli/v3"
)

type Client struct {
	Self          *Peer
	Hostname      string
	KnownPeers    map[string]*Peer
	SelectedFiles []FileInfo
	MU            sync.RWMutex
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
		KnownPeers:    make(map[string]*Peer),
		SelectedFiles: make([]FileInfo, 6),
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
				Name:    "listen",
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

					fmt.Printf("Listening for incoming files. Files will be saved to %s\n", downloadDir)

					func() {
						go c.listen(ctx)
						go c.broadcastPresence(ctx)
					}()

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

	msg := Message{
		Type:       TypeTransferReq,
		SenderID:   c.Self.ID,
		SenderName: c.Self.Name,
		IPAddress:  c.Self.IPAddress,
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

	time.Sleep(1 * time.Second)

	tcpConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", peer.IPAddress, transferPort))
	if err != nil {
		fmt.Printf("Error connecting to peer for file transfer: %v\n", err)
		return
	}
	defer tcpConn.Close()

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

			option := fmt.Sprintf("%s (%s)", entry.Name(), formatSize(fileInfo.Size()))
			fileOptions = append(fileOptions, huh.NewOption(option, entry.Name()))
		}
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

	conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	buffer := make([]byte, 1024)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))

			n, remoteAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				fmt.Printf("Error reading from UDP: %v\n", err)
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
				c.handleTransferRequest(msg)
			}
		}
	}
}

func (c *Client) refreshPeers() {
	fmt.Println(INFO.Render("Refreshing peer list..."))

	c.MU.Lock()
	c.KnownPeers = make(map[string]*Peer)
	c.MU.Unlock()

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

	c.MU.RLock()
	if len(c.KnownPeers) > 0 {
		fmt.Printf(INFO.Render("Found %d peers on the network.\n"), len(c.KnownPeers))
	} else {
		fmt.Println(INFO.Render("No peers found."))
	}
	c.MU.RUnlock()
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

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	c.sendDiscoveryBroadcast(conn)

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

	go func() {
		c.listen(ctx)
		c.broadcastPresence(ctx)
	}()

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

	_, err = conn.Write(jsonData)
	if err != nil {
		fmt.Printf("Error sending discovery broadcast: %v\n", err)
	}
}

func (c *Client) listenWithTimeout(ctx context.Context) {
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

	conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	buffer := make([]byte, 1024)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))

			n, remoteAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				fmt.Printf("Error reading from UDP: %v\n", err)
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
			}
		}
	}
}

func (c *Client) discoverPeers(timeoutSeconds int) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	go c.listenWithTimeout(ctx)

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

	<-ctx.Done()
}

func (c *Client) displayPeers() {
	c.MU.RLock()
	defer c.MU.RUnlock()

	if len(c.KnownPeers) == 0 {
		fmt.Println(INFO.Render("No peers found on the network."))
		return
	}

	fmt.Printf("len %d", len(c.KnownPeers))

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

func (c *Client) handleTransferRequest(msg Message) {
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
