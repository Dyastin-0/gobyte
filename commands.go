package gobyte

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/urfave/cli/v3"
)

func (c *Client) chuckCommand(ctx context.Context, cmd *cli.Command) error {
	dir := cmd.String("dir")

	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		fmt.Println(ERROR.Bold(true).Render("-d does not exists"))
		c.Shutdown <- syscall.SIGINT
		return nil
	}

	name := cmd.String("name")
	if name != "" {
		c.self.Name = name
	}

	go c.runInteractiveMode(ctx, dir)

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
