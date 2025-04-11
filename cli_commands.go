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
		fmt.Println(TITLE.Render("GOBYTE"))
		fmt.Printf("Running as: %s (%s)\n", c.Self.Name, c.Self.IPAddress)
		c.runInteractiveMode(ctx, cancel)

		return nil
	}
}

func (c *Client) listCommand(ctx context.Context, cmd *cli.Command) error {
	fmt.Println("Discovering peers...")
	c.discoverPeers(2)
	c.displayPeers()
	return nil
}

func (c *Client) sendCommand(ctx context.Context, cmd *cli.Command) error {
	files, err := resolveFiles(cmd.StringSlice("file"))
	if err != nil {
		return err
	}
	if cmd.Bool("interactive") {
		c.discoverPeers(2)
		peers, err := c.selectPeers()
		if err != nil || len(peers) == 0 {
			return fmt.Errorf("no peers selected")
		}
		for _, peer := range peers {
			c.sendFilesTo(&peer, files)
		}
	} else if len(cmd.StringSlice("peer")) > 0 {
		c.discoverPeers(2)
		for _, peerID := range cmd.StringSlice("peer") {
			c.MU.RLock()
			peer, exists := c.KnownPeers[peerID]
			c.MU.RUnlock()
			if !exists {
				fmt.Printf("Peer ID %s not found\n", peerID)
				continue
			}
			c.sendFilesTo(peer, files)
		}
	} else {
		return fmt.Errorf("must specify peers with --peer or use --interactive")
	}
	return nil
}

func (c *Client) chompCommand(ctx context.Context, cmd *cli.Command) error {
	downloadDir := cmd.String("dir")
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return err
	}
	fmt.Println(INFO.Render(fmt.Sprintf("Listening for incoming files. Files will be saved to %s", downloadDir)))
	go c.handleTransferRequests(ctx, downloadDir)
	go c.listen(ctx)
	go c.broadcastPresence(ctx)
	<-ctx.Done()
	return nil
}
