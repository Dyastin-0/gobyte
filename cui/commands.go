package cui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Dyastin-0/gobyte/styles"
	"github.com/urfave/cli/v3"
)

func (cui *ClientUI) chuckCommand(ctx context.Context, cmd *cli.Command) error {
	name := cmd.String("name")
	if name != "" {
		cui.client.Self.Name = name
	}

	fmt.Println(styles.TITLE.Render("gobyte"), styles.SUCCESS.Render(fmt.Sprintf("as %s (%s)", cui.client.Self.Name, cui.client.Self.IPAddress)))

	dir := cmd.String("dir")

	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		fmt.Println(styles.ERROR.Bold(true).Render("-d does not exists"))
		return err
	}

	cui.menu(dir)

	return nil
}

func (cui *ClientUI) chompCommand(ctx context.Context, cmd *cli.Command) error {
	name := cmd.String("name")
	if name != "" {
		cui.client.Self.Name = name
	}

	fmt.Println(styles.TITLE.Render("gobyte"), styles.SUCCESS.Render(fmt.Sprintf("as %s (%s)", cui.client.Self.Name, cui.client.Self.IPAddress)))

	dir := cmd.String("dir")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	fmt.Println(styles.INFO.Render(fmt.Sprintf("listening for requests. files will be saved at %s", dir)))

	go cui.chomp(ctx, dir)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	<-shutdown

	return nil
}
