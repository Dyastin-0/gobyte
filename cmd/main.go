package main

import (
	"context"

	"github.com/Dyastin-0/gobyte"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	client := gobyte.NewClient(ctx)

	client.Run(ctx, cancel)
}
