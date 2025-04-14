package main

import (
	"os"

	"github.com/urfave/cli/v3"
)

func (c *Client) NewCLI() *cli.Command {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	return &cli.Command{
		Name:    "gobyte",
		Usage:   "Blazingly fast local area network file sharing CLI app",
		Version: "0.1.0",
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
					&cli.StringFlag{
						Name:    "dir",
						Aliases: []string{"d"},
						Usage:   "Overrided the default initial directory",
						Value:   homeDir,
					},
				},
				Action: c.chuckCommand,
			},
			{
				Name:  "chomp",
				Usage: "Listen for incoming requests",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "name",
						Aliases: []string{"n"},
						Usage:   "Override the default device name",
					},
					&cli.StringFlag{
						Name:    "dir",
						Aliases: []string{"d"},
						Usage:   "Directory to receive files to",
						Value:   homeDir + "/gobyte/received",
					},
				},
				Action: c.chompCommand,
			},
		},
	}
}
