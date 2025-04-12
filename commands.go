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
			c.self.Name = cmd.String("name")
		}
		fmt.Printf("Running as: %s (%s)\n", c.self.Name, c.self.IPAddress)
		c.runInteractiveMode(ctx, cancel)

		return nil
	}
}

func (c *Client) chompCommand(ctx context.Context, cmd *cli.Command) error {
	dir := cmd.String("dir")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	fmt.Println(INFO.Render(fmt.Sprintf("Listening for incoming files. Files will be saved to %s", dir)))
	go c.chomp(ctx, dir)
	go c.pingBroadcaster(ctx)
	go c.listen(ctx)
	go c.broadcastPresence(ctx)
	<-ctx.Done()
	return nil
}
