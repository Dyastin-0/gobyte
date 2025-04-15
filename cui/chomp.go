package cui

import (
	"context"
	"fmt"
	"time"

	"github.com/Dyastin-0/gobyte/types"
)

func (ui *ClientUI) chomp(ctx context.Context, dir string) {
	ui.client.StartChompListener(ctx, dir, func(msg types.Message) (bool, error) {
		str := "file"
		if len(msg.Files) > 1 {
			str += "s"
		}

		return ui.showConfirm(
			fmt.Sprintf("accept %d %s from %s?", len(msg.Files), str, msg.SenderName),
			15*time.Second,
		)
	})
}
