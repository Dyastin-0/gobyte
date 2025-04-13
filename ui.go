package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/huh"
)

func (c *Client) runInteractiveMode(ctx context.Context, dir string) {
	go c.listen(ctx)
	go c.pingBroadcaster(ctx)

	for {
		option := c.showMainMenu()

		switch option {
		case "send":
			c.chuck(dir)

		case "peers":
			c.displayPeers()

		case "quit":
			return
		}
	}
}

func (c *Client) showConfirm(title string, duration time.Duration) (bool, error) {
	var confirm bool

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Affirmative("yes").
				Negative("no").
				Title(title).
				Value(&confirm),
		),
	).WithTimeout(duration)

	err := form.Run()
	if err != nil {
		return false, err
	}

	return confirm, nil
}

func (c *Client) showMainMenu() string {
	var option string

	c.mu.RLock()
	peerCount := len(c.knownPeers)
	c.mu.RUnlock()

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("what would you like to do?").
				Options(
					huh.NewOption(fmt.Sprintf("send files (%d peers available)", peerCount), "send"),
					huh.NewOption("list peers", "peers"),
					huh.NewOption("quit", "quit"),
				).
				Value(&option),
		),
	)

	err := form.Run()
	if err != nil {
		return "quit"
	}
	return option
}

func (c *Client) selectPeers() ([]Peer, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.knownPeers) == 0 {
		return nil, fmt.Errorf("no peers found")
	}

	var peerOptions []huh.Option[string]
	peerMap := make(map[string]Peer)

	for _, peer := range c.knownPeers {
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

func (c *Client) selectFiles(dir string) ([]FileInfo, error) {
	selectedFiles := make(map[string]FileInfo)
	currentDir := dir
	var selected string

	for {
		entries, err := os.ReadDir(currentDir)
		if err != nil {
			return nil, err
		}

		var options []huh.Option[string]

		options = append(options, huh.NewOption("../", "../"))

		for _, entry := range entries {
			if entry.IsDir() {
				options = append(options, huh.NewOption(entry.Name()+"/", entry.Name()))
			}
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				fileInfo, enterr := entry.Info()
				if enterr != nil {
					continue
				}

				name := entry.Name()
				fullPath := filepath.Join(currentDir, name)
				displayName := name

				if _, selected := selectedFiles[fullPath]; selected {
					displayName = SUCCESS.Render("[✓] " + name)
				}

				option := fmt.Sprintf("%s (%d bytes)", displayName, fileInfo.Size())
				options = append(options, huh.NewOption(option, name))
			}
		}

		options = append(options, huh.NewOption("done", "done"))
		options = append(options, huh.NewOption("cancel", "cancel"))

		if len(options) == 2 {
			return nil, fmt.Errorf("directory is empty: %s", currentDir)
		}

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(fmt.Sprintf("browsing: %s (%d files selected)", currentDir, len(selectedFiles))).
					Options(options...).
					Value(&selected),
			),
		)

		err = form.Run()
		if err != nil {
			return nil, err
		}

		switch selected {
		case "..":
			currentDir = filepath.Dir(currentDir)
			continue

		case "cancel":
			return nil, fmt.Errorf("file selection cancelled")

		case "done":
			if len(selectedFiles) == 0 {
				return nil, fmt.Errorf("no file selected")
			}

			var result []FileInfo
			for _, fileInfo := range selectedFiles {
				result = append(result, fileInfo)
			}
			return result, nil

		default:
			fullPath := filepath.Join(currentDir, selected)
			fileInfo, err := os.Stat(fullPath)
			if err != nil {
				return nil, fmt.Errorf("failed to access %s", fullPath)
			}

			if fileInfo.IsDir() {
				currentDir = fullPath
			} else {
				if _, exists := selectedFiles[fullPath]; exists {
					delete(selectedFiles, fullPath)
				} else {
					selectedFiles[fullPath] = FileInfo{
						Name: selected,
						Size: fileInfo.Size(),
						Path: fullPath,
					}
				}
			}
		}

	}
}

func (c *Client) displayPeers() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.knownPeers) == 0 {
		fmt.Println(INFO.Render("no peers found"))
		return
	}

	fmt.Println(INFO.Render("peers:"))

	for _, peer := range c.knownPeers {
		fmt.Println(SUCCESS.Render(fmt.Sprintf("%s (%s)", peer.Name, peer.IPAddress)))
	}
}
