package main

import (
	"context"
	"time"

	"github.com/Dyastin-0/gobyte"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	defer func() {
		cancel()
		time.Sleep(100 * time.Millisecond)
	}()

	client := gobyte.NewClient(ctx)
	client.Run(ctx)
}
