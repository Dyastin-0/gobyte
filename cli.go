package gobyte

import (
	"context"

	"github.com/urfave/cli/v3"
)

func (c *Client) createCLIApp(cancel context.CancelFunc) *cli.Command {
	return &cli.Command{
		Name:        "gobyte",
		Usage:       "Share files on your local network",
		Version:     "1.0.0",
		Description: "A command-line tool for sharing files with peers on your local network",
		Commands: []*cli.Command{
			{
				Name:    "chuck",
				Aliases: []string{"ck"},
				Usage:   "Send files to discovered peers",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "name",
						Aliases: []string{"n"},
						Usage:   "Override the default device name",
					},
				},
				Action: c.chuckCommand(cancel),
			},
			{
				Name:    "list",
				Aliases: []string{"ls", "l"},
				Usage:   "List available peers on the network",
				Action:  c.listCommand,
			},
			{
				Name:    "send",
				Aliases: []string{"s"},
				Usage:   "Send files to peers",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:     "file",
						Aliases:  []string{"f"},
						Usage:    "File(s) to send (repeat flag for multiple files)",
						Required: true,
					},
					&cli.StringSliceFlag{
						Name:    "peer",
						Aliases: []string{"p"},
						Usage:   "Peer ID(s) to send to (repeat flag for multiple peers)",
					},
					&cli.BoolFlag{
						Name:    "interactive",
						Aliases: []string{"i"},
						Usage:   "Use interactive mode to select peers",
					},
				},
				Action: c.sendCommand,
			},
			{
				Name:    "chomp",
				Aliases: []string{"l"},
				Usage:   "Listen for incoming files",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "dir",
						Aliases: []string{"d"},
						Usage:   "Directory to save received files",
						Value:   "./files",
					},
				},
				Action: c.chompCommand,
			},
		},
	}
}
