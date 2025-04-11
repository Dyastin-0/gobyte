package gobyte

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

func (c *Client) chuckCommand(cancel context.CancelFunc) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		if cmd.String("name") != "" {
			c.Self.Name = cmd.String("name")
		}
		fmt.Printf("Running as: %s (%s)\n", c.Self.Name, c.Self.IPAddress)
		c.runInteractiveMode(ctx, cancel)

		return nil
	}
}

func (c *Client) listCommand(ctx context.Context, cmd *cli.Command) error {
	fmt.Println("Discovering peers...")
	// c.discoverPeers(2)
	c.displayPeers()
	return nil
}

func (c *Client) chompCommand(ctx context.Context, cmd *cli.Command) error {
	downloadDir := cmd.String("dir")
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return err
	}
	fmt.Println(INFO.Render(fmt.Sprintf("Listening for incoming files. Files will be saved to %s", downloadDir)))
	go c.handleChomping(ctx, downloadDir)
	go c.listen(ctx)
	go c.broadcastPresence(ctx)
	<-ctx.Done()
	return nil
}
