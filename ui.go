package gobyte

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
)

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
