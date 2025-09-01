// Package cmd ...
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Dyastin-0/gobyte/core"
	"github.com/common-nighthawk/go-figure"
	"github.com/urfave/cli/v3"
)

const defaultDir = "gobyte/received"

func New() *cli.Command {
	return &cli.Command{
		Name:    "gobyte",
		Usage:   "a simple p2p local area network file sharing cli app",
		Version: core.VERSION,
		Action:  gobyteAction,
		Commands: []*cli.Command{
			sendCommand(),
			receiveCommand(),
		},
	}
}

func gobyteAction(ctx context.Context, cmd *cli.Command) error {
	figure := figure.NewFigure("gobyte-cli", "", true)
	figure.Print()

	fmt.Println()

	err := cli.ShowAppHelp(cmd)
	if err != nil {
		panic(err)
	}

	return nil
}

func defaultFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "addr",
			Aliases: []string{"a"},
			Value:   ":8080",
		},
		&cli.StringFlag{
			Name:    "dir",
			Aliases: []string{"d"},
			Value:   homeDir(),
		},
		&cli.StringFlag{
			Name:    "bAddr",
			Aliases: []string{"b"},
			Value:   ":42069",
		},
	}
}

func sendCommand() *cli.Command {
	return &cli.Command{
		Name:   "send",
		Usage:  "run as a sender",
		Flags:  defaultFlags(),
		Action: sendAction,
	}
}

func sendAction(ctx context.Context, cmd *cli.Command) error {
	addr := cmd.String("addr")
	dir := cmd.String("dir")
	baddr := cmd.String("bAddr")

	s := core.NewSenderClient(addr, baddr, dir)

	return s.StartSender(ctx)
}

func receiveCommand() *cli.Command {
	return &cli.Command{
		Name:   "receive",
		Usage:  "run as a receiver",
		Flags:  defaultFlags(),
		Action: receiveAction,
	}
}

func receiveAction(ctx context.Context, cmd *cli.Command) error {
	addr := cmd.String("addr")
	dir := cmd.String("dir")
	if dir == homeDir() {
		dir = filepath.Join(dir, defaultDir)
	}
	baddr := cmd.String("bAddr")

	r := core.NewReceiverClient(addr, baddr, dir)

	errch := make(chan error, 1)

	go func() {
		errch <- r.StartReceiver(ctx)
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errch:
		return err
	}
}

func homeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "./"
	}

	return homeDir
}
