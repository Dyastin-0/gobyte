package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

func (c *Client) chuckCommand(ctx context.Context, cmd *cli.Command) error {
	fmt.Println(TITLE.Render("gobyte"), SUCCESS.Render(fmt.Sprintf("as %s (%s)", c.self.Name, c.self.IPAddress)))

	dir := cmd.String("dir")

	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		fmt.Println(ERROR.Bold(true).Render("-d does not exists"))
		return nil
	}

	name := cmd.String("name")
	if name != "" {
		c.self.Name = name
	}

	c.runInteractiveMode(ctx, dir)

	fmt.Println(TITLE.Render("gobyte some grass"))

	return nil
}

func (c *Client) chompCommand(ctx context.Context, cmd *cli.Command) error {
	fmt.Println(TITLE.Render("gobyte"), SUCCESS.Render(fmt.Sprintf("as %s (%s)", c.self.Name, c.self.IPAddress)))

	dir := cmd.String("dir")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	fmt.Println(INFO.Render(fmt.Sprintf("Listening for requests. Files will be saved at %s", dir)))

	go c.chomp(ctx, dir)
	go c.pingBroadcaster(ctx)
	go c.presenceBroadcaster(ctx)

	c.listen(ctx)

	fmt.Println(TITLE.Render("gobyte some grass"))

	return nil
}
