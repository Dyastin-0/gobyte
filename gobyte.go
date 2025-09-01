package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/Dyastin-0/gobyte/cmd"
	"github.com/charmbracelet/huh"
)

func main() {
	c := cmd.New()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	if err := c.Run(ctx, os.Args); err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, huh.ErrUserAborted) {
			panic(err)
		}
	}
}
