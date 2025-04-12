package gobyte

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

func (c *Client) chuckCommand(ctx context.Context, cmd *cli.Command) error {
	if cmd.String("name") != "" {
		c.self.Name = cmd.String("name")
	}

	c.runInteractiveMode(ctx)

	return nil
}

func (c *Client) chompCommand(ctx context.Context, cmd *cli.Command) error {
	dir := cmd.String("dir")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	fmt.Println(INFO.Render(fmt.Sprintf("Listening for requests. Files will be saved at %s", dir)))

	go c.chomp(ctx, dir)
	go c.pingBroadcaster(ctx)
	go c.presenceBroadcaster(ctx)

	c.listen(ctx)

	return nil
}
