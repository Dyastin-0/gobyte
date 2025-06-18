package cui

import (
	"context"
	"fmt"
	"time"

	"github.com/Dyastin-0/gobyte/styles"
	"github.com/Dyastin-0/gobyte/types"
)

func (cui *ClientUI) chomp(ctx context.Context, dir string) {
	onNewPeer := func(peerID string, fingerprint string) bool {
		fmt.Println(styles.WARNING.Bold(true).Render("warning!"))
		fmt.Printf("the authenticity of peer %s can't be established.\n", peerID)
		fmt.Printf("tls certificate fingerprint is %s.\n", fingerprint)

		trusted, err := cui.showConfirm("do you trust this peer?", 15*time.Second)
		if err != nil {
			return false
		}

		return trusted
	}

	onRequest := func(msg types.Message) (bool, error) {
		str := "file"
		if msg.Len > 1 {
			str += "s"
		}

		return cui.showConfirm(
			fmt.Sprintf("accept %d %s from %s?", msg.Len, str, msg.SenderName),
			15*time.Second,
		)
	}

	cui.client.StartChompListener(ctx, dir, onNewPeer, onRequest)
}
