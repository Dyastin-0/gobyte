package gobyte

import (
	"context"

	"github.com/urfave/cli/v3"
)

func (c *Client) createCLIApp(cancel context.CancelFunc) *cli.Command {
	return &cli.Command{
		Name:        "gobyte",
		Usage:       "Blazingly fast local LAN file sharing CLI app",
		Version:     "0.0.1",
		Description: "A command-line tool for sharing files with peers on your local network",
		Commands: []*cli.Command{
			{
				Name:  "chuck",
				Usage: "Send files to your local peers",
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
				Name:  "chomp",
				Usage: "Listen for incoming requests",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "dir",
						Aliases: []string{"d"},
						Usage:   "Directory to receive files to",
						Value:   "./files",
					},
				},
				Action: c.chompCommand,
			},
		},
	}
}
