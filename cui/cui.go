package cui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Dyastin-0/gobyte/client"
	"github.com/Dyastin-0/gobyte/styles"
	"github.com/Dyastin-0/gobyte/types"
	"github.com/charmbracelet/huh"
)

type ClientUI struct {
	client *client.Client
}

func New(client *client.Client) *ClientUI {
	return &ClientUI{
		client,
	}
}

func (cui *ClientUI) showMainMenu() string {
	var option string
	count, _ := cui.client.CountKnownPeers()

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("what would you like to do?").
				Options(
					huh.NewOption(fmt.Sprintf("chuck files (%d chompers available)", count), "send"),
					huh.NewOption("list chompers", "peers"),
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

func (cui *ClientUI) showConfirm(title string, duration time.Duration) (bool, error) {
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

func (cui *ClientUI) selectPeers() ([]*types.Peer, error) {
	count, peers := cui.client.CountKnownPeers()

	if count == 0 {
		return nil, fmt.Errorf("no chompers discovered")
	}

	var peerOptions []huh.Option[string]

	for _, peer := range peers {
		option := fmt.Sprintf("%s (%s)", peer.Name, peer.IPAddress)
		peerOptions = append(peerOptions, huh.NewOption(option, peer.ID))
	}

	var selectedPeerIDs []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("select chomper").
				Options(peerOptions...).
				Value(&selectedPeerIDs),
		),
	)

	err := form.Run()
	if err != nil {
		return nil, err
	}

	var selectedPeers []*types.Peer
	for _, id := range selectedPeerIDs {
		selectedPeers = append(selectedPeers, peers[id])
	}

	return selectedPeers, nil
}

func (cui *ClientUI) selectFiles(dir string) ([]types.FileInfo, error) {
	selectedFiles := make(map[string]types.FileInfo)
	currentDir := dir
	var selected string

	for {
		entries, err := os.ReadDir(currentDir)
		if err != nil {
			return nil, err
		}

		var options []huh.Option[string]

		options = append(options, huh.NewOption("../", "../"))
		options = formatAndAppendEntries(options, entries, selectedFiles, currentDir)
		options = append(options, huh.NewOption("done", "done"))
		options = append(options, huh.NewOption("cancel", "cancel"))

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
		case "cancel":
			return nil, fmt.Errorf("file selection cancelled")

		case "done":
			if len(selectedFiles) == 0 {
				return nil, fmt.Errorf("no file selected")
			}

			var result []types.FileInfo
			for _, fileInfo := range selectedFiles {
				result = append(result, fileInfo)
			}
			return result, nil

		default:
			fullPath := filepath.Join(currentDir, selected)
			fileInfo, err := os.Stat(fullPath)
			if err != nil {
				fmt.Println(styles.ERROR.Render(fmt.Sprintf("failed to access %s: %v", fullPath, err)))
				continue
			}

			if fileInfo.IsDir() {
				currentDir = fullPath
				continue
			}

			if _, exists := selectedFiles[fullPath]; exists {
				delete(selectedFiles, fullPath)
			} else {
				selectedFiles[fullPath] = types.FileInfo{
					Name: selected,
					Size: fileInfo.Size(),
					Path: fullPath,
				}
			}
		}

	}
}

func (cui *ClientUI) displayPeers() {
	count, peers := cui.client.CountKnownPeers()

	if count == 0 {
		fmt.Println(styles.INFO.Render("no chompers found"))
		return
	}

	fmt.Println(styles.INFO.Render("chompers"))

	for _, peer := range peers {
		fmt.Println(styles.SUCCESS.PaddingLeft(2).Render(fmt.Sprintf("%s (%s)", peer.Name, peer.IPAddress)))
	}
}

func formatAndAppendEntries(options []huh.Option[string], entries []os.DirEntry, selectedFiles map[string]types.FileInfo, currentDir string) []huh.Option[string] {
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
				displayName = styles.SUCCESS.Render("[✓] " + name)
			}

			option := fmt.Sprintf("%s (%d bytes)", displayName, fileInfo.Size())
			options = append(options, huh.NewOption(option, name))
		}
	}

	return options
}
