package cui

import (
	"context"
	"fmt"
	"time"

	"github.com/Dyastin-0/gobyte/styles"
	"github.com/Dyastin-0/gobyte/types"
)

func (cui *ClientUI) chomp(ctx context.Context, dir string) {
	cui.client.StartChompListener(ctx, dir, func(msg types.Message) (bool, error) {
		str := "file"
		if len(msg.Files) > 1 {
			str += "s"
		}

		fmt.Println(styles.INFO.Render("files"))
		for _, fileInfo := range msg.Files {
			fmt.Println(styles.INFO.PaddingLeft(2).Render(fmt.Sprintf("%s (%d)", fileInfo.Name, fileInfo.Size)))
		}

		return cui.showConfirm(
			fmt.Sprintf("accept %d %s from %s?", len(msg.Files), str, msg.SenderName),
			15*time.Second,
		)
	})
}
