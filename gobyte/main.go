package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Dyastin-0/gobyte"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	defer func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	client := gobyte.NewClient(ctx)

	go client.Run(ctx)

	<-client.Shutdown

	fmt.Println(gobyte.TITLE.Render("gobyte some grass"))
}
