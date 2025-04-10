package gobyte

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

func showMainMenu() string {
	var option string

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
