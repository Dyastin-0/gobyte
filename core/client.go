package core

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"

	"github.com/Dyastin-0/gobyte/tofu"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	ErrRequestDenied   = errors.New("request denied")
	ErrInvalidResponse = errors.New("invalid response")

	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

type Client struct {
	addr         string
	sender       *Sender
	receiver     *Receiver
	broadcaster  *Broadcaster
	fileselector *FileSelector
	peerselector *PeerSelector
	tofu         *tofu.Tofu

	onRequest func(*Request) bool
}

func NewSenderClient(addr, baddr, dir string) *Client {
	if dir == "" {
		dir = "./"
	}

	addr = fmt.Sprintf("%s%s", outboundIP().String(), addr)

	return &Client{
		addr:         addr,
		broadcaster:  NewBroadcaster(baddr, addr),
		sender:       NewSender(),
		fileselector: NewFileSelector(dir),
		peerselector: NewPeerSelector(nil),
		tofu:         tofu.New(hostname()),
	}
}

func NewReceiverClient(addr, baddr, dir string) *Client {
	if dir == "" {
		dir = "./"
	}

	addr = fmt.Sprintf("%s%s", outboundIP().String(), addr)

	return &Client{
		addr:         addr,
		broadcaster:  NewBroadcaster(baddr, addr),
		receiver:     NewReceiver(dir),
		fileselector: NewFileSelector(dir),
		peerselector: NewPeerSelector(nil),
		tofu:         tofu.New(hostname()),
		onRequest:    OnRequest,
	}
}

func (c *Client) StartReceiver(ctx context.Context) error {
	err := c.tofu.Init()
	if err != nil {
		return err
	}

	// Override default tofu.OnNewPeer
	c.tofu.OnNewPeer = OnNewPeer

	ln, err := c.tofu.Listen(c.addr)
	if err != nil {
		return err
	}

	// Only send out broadcasts
	err = c.broadcaster.Init()
	if err != nil {
		return err
	}
	go c.broadcaster.b(ctx)

	err = c.listen(ctx, ln)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) StartSender(ctx context.Context) error {
	cancelContext, cancel := context.WithCancel(ctx)
	defer cancel()

	err := c.tofu.Init()
	if err != nil {
		return err
	}

	// Override default tofu.OnNewPeer
	c.tofu.OnNewPeer = OnNewPeer

	go c.broadcaster.Start(cancelContext)

	// Set the peers to selector so we can display them
	c.peerselector.peers = c.broadcaster.peers

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err := c.peerselector.RunRecur()
			if err != nil {
				return err
			}

			if len(c.peerselector.Selected) == 0 {
				if Continue("No peers were selected, try again?") {
					continue
				}

				return nil
			}

			err = c.fileselector.RunRecur()
			if err != nil {
				return err
			}

			if len(c.fileselector.Selected) == 0 {
				if Continue("No files were selected, try again?") {
					continue
				}

				return nil
			}

			// TODO: Handle multiple peers concurrently
			// How should we display the bars when sending to multiple peers?
			// How should we display forms when sending to multiple peers at first time?
			for _, p := range c.peerselector.Selected {
				conn, err := c.tofu.Dial(p.data)
				if err != nil {
					return err
				}

				req := NewRequest(
					uint64(c.fileselector.nBytesSelected),
					uint32(len(c.fileselector.Selected)),
				)

				err = c.sender.WriteRequest(conn, req)
				if err != nil {
					return err
				}

				err = c.sender.ReadResponse(conn)
				if err != nil {
					return err
				}

				err = c.sender.Send(conn, c.fileselector.Selected, req)
				if err != nil {
					return err
				}

				err = c.sender.WriteEnd(conn)
				if err != nil {
					log.Println("[warn] failed to write end, but all files were written")
					return err
				}

				conn.Close()
			}

			if !Continue("Do you want to send again? (Yes/No)") {
				return nil
			}
		}
	}
}

func (c *Client) listen(ctx context.Context, ln net.Listener) error {
	fmt.Println(pageStyle.Render("Listening on " + c.addr))

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if errors.Is(err, net.ErrClosed) {
				return nil
			}

			log.Printf("[warn] Accept error: %v", err)
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()
			err := c.receiver.receive(conn)
			if err != nil {
				log.Printf("[err] Connection handler error: %v", err)
			}
		}(conn)
	}
}

func Continue(txt string) bool {
	var confirm bool

	huh.NewConfirm().
		Title(txt).
		Affirmative("Yes").
		Negative("No").
		Value(&confirm).
		Run()

	return confirm
}

func OnNewPeer(id, fingerprint string) bool {
	confirm := false

	title := warningStyle.Render(fmt.Sprintf("The authenticity of peer '%s' can't be established.\nCertificate fingerprint is\n%s\nDo you trust this peer?", id, fingerprint))

	huh.NewConfirm().
		Title(title).
		Affirmative("Yes").
		Negative("No").
		Value(&confirm).
		Run()

	return confirm
}
