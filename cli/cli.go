package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Dyastin-0/gobyte"
	"github.com/urfave/cli/v3"
)

func main() {
	c := New()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	if err := c.Run(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}

func New() *cli.Command {
	return &cli.Command{
		Name:   "gobyte",
		Usage:  "a simple p2p local area network file sharing cli app",
		Action: gobyteAction,
		Commands: []*cli.Command{
			sendCommand(),
			receiveCommand(),
		},
	}
}

func gobyteAction(ctx context.Context, cmd *cli.Command) error {
	return nil
}

func defaultFlags() []cli.Flag {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "./"
	}

	return []cli.Flag{
		&cli.StringFlag{
			Name:    "addr",
			Aliases: []string{"a"},
			Value:   ":8080",
		},
		&cli.StringFlag{
			Name:    "dir",
			Aliases: []string{"d"},
			Value:   homeDir,
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

	s := gobyte.NewSenderClient(addr, baddr, dir)

	return s.StartSender(ctx)
}

func receiveCommand() *cli.Command {
	return &cli.Command{
		Name:   "receive",
		Usage:  "run as a receier",
		Flags:  defaultFlags(),
		Action: receiveAction,
	}
}

func receiveAction(ctx context.Context, cmd *cli.Command) error {
	addr := cmd.String("addr")
	dir := cmd.String("dir")
	baddr := cmd.String("bAddr")

	r := gobyte.NewReceiverClient(addr, baddr, dir)

	errch := make(chan error, 1)

	go func() {
		errch <- r.StartReceiver(ctx)
	}()

	<-ctx.Done()
	close(errch)

	return <-errch
}
