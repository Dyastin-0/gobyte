package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Dyastin-0/gobyte/client"
	"github.com/Dyastin-0/gobyte/cui"
	"github.com/Dyastin-0/gobyte/styles"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	defer func() {
		cancel()
		fmt.Println(styles.TITLE.Render("gobyte some grass"))
		time.Sleep(100 * time.Millisecond)
	}()

	client := client.New(ctx)

	clientUI := cui.New(client)
	err := clientUI.Run(ctx)
	if err != nil {
		panic(err)
	}
}
