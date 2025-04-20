package cui

import (
	"context"
	"fmt"
	"time"

	"github.com/Dyastin-0/gobyte/styles"
	"github.com/Dyastin-0/gobyte/types"
)

func (cui *ClientUI) chomp(ctx context.Context, dir string) {
	onNewPeer := func(peerID string) bool {
		trusted, err := cui.showConfirm(fmt.Sprintf("Add %s on trusted peers?", peerID), 15*time.Second)
		if err != nil {
			return false
		}

		return trusted
	}

	onRequest := func(msg types.Message) (bool, error) {
		str := "file"
		if len(msg.Files) > 1 {
			str += "s"
		}

		fmt.Println(styles.INFO.Render("files"))
		for _, fileInfo := range msg.Files {
			fmt.Println(styles.INFO.PaddingLeft(2).Render(fmt.Sprintf("%s (%d bytes)", fileInfo.Name, fileInfo.Size)))
		}

		return cui.showConfirm(
			fmt.Sprintf("accept %d %s from %s?", len(msg.Files), str, msg.SenderName),
			15*time.Second,
		)
	}

	cui.client.StartChompListener(ctx, dir, onNewPeer, onRequest)
}
