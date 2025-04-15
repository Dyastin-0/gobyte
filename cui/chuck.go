package cui

import (
	"fmt"

	"github.com/Dyastin-0/gobyte/styles"
)

func (cui *ClientUI) menu(dir string) {
	for {
		option := cui.showMainMenu()

		switch option {
		case "send":
			cui.chuck(dir)

		case "peers":
			cui.displayPeers()

		case "quit":
			return
		}
	}
}

func (cui *ClientUI) chuck(dir string) {
	peers, err := cui.selectPeers()
	if err != nil || len(peers) == 0 {
		fmt.Println(styles.INFO.Render("no peers to send to"))
		return
	}

	files, err := cui.selectFiles(dir)
	if err != nil || len(files) == 0 {
		fmt.Println(styles.INFO.Render(fmt.Sprintf("%v", err)))
		return
	}

	err = cui.client.ChuckFilesToPeers(peers, files)
	if err != nil {
		fmt.Println(styles.ERROR.Render(err.Error()))
	}
}
