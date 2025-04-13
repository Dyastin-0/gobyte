package main

import (
	"context"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	defer func() {
		cancel()
		time.Sleep(100 * time.Millisecond)
	}()

	client := NewClient(ctx)
	client.Run(ctx)
}
