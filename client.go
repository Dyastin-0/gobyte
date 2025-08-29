package gobyte

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/Dyastin-0/gobyte/tofu"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	ErrRequestDenied = errors.New("request denied")

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

	onRequest func(*RequestHeader) bool
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
		receiver:     NewReceiver(addr),
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

	go c.broadcaster.Start(ctx)

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

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			c.peerselector.peers = c.broadcaster.peers
			err := c.peerselector.RunRecur()
			if err != nil {
				return err
			}

			if len(c.peerselector.Selected) == 0 {
				if Continue("No peers were selected, try again? (Yes/No)") {
					continue
				}

				return nil
			}

			err = c.fileselector.RunRecur()
			if err != nil {
				return err
			}

			if len(c.fileselector.Selected) == 0 {
				if Continue("No files were selected, try again? (Yes/No)") {
					continue
				}

				return nil
			}

			for _, p := range c.peerselector.Selected {
				conn, err := c.tofu.Dial(p.data)
				if err != nil {
					return err
				}

				r := &RequestHeader{
					n:      len(c.fileselector.Selected),
					nbytes: c.fileselector.nBytesSelected,
				}

				err = c.WriteRequest(conn, r)
				if err != nil {
					return err
				}

				summ, err := c.sender.Send(conn, c.fileselector.Selected)
				if err != nil {
					return err
				}

				log.Printf("Sent bytes: %d\n", summ.nBytes)
				log.Printf("Sent files: %d\n", len(summ.files))
				log.Printf("Failed bytes: %d\n", summ.nFailedBytes)
				log.Printf("Failed files: %d\n", len(summ.failedFiles))
			}

			if !Continue("Do you want to send again? (Yes/No)") {
				return nil
			}
		}
	}
}

func (c *Client) WriteRequest(conn net.Conn, r *RequestHeader) error {
	encoded, err := r.Encoded()
	if err != nil {
		return err
	}

	encodedBytes, ok := encoded.(*EncodedRequestHeader)
	if !ok {
		return errors.New("faild to type assert to EncodedRequestHeader")
	}

	_, err = conn.Write(*encodedBytes)
	if err != nil {
		return err
	}

	confirm, err := c.ReadResponse(conn)
	if err != nil {
		return err
	}

	if *confirm {
		return nil
	}

	return ErrRequestDenied
}

func (c *Client) ReadResponse(conn net.Conn) (*bool, error) {
	rd := bufio.NewReader(conn)

	response, err := rd.ReadString(delim)
	if err != nil {
		return nil, err
	}

	encodedHeader := EncodedResponseHeader(response)
	header, err := encodedHeader.Parse()
	if err != nil {
		return nil, err
	}

	parsedHeader, ok := header.(*ResponseHeader)
	if !ok {
		return nil, errors.New("faild to type assert to ResponseHeader")
	}

	ok = parsedHeader.ok == ResponseOk
	return &ok, nil
}

func (c *Client) listen(ctx context.Context, ln net.Listener) error {
	fmt.Println(pageStyle.Italic(true).Render("Listening on port " + c.addr))

	// Close listener when context is cancelled
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
			err := c.handleConn(conn)
			if err != nil {
				log.Printf("[err] Connection handler error: %v", err)
			}
		}(conn)
	}
}

func (c *Client) handleConn(conn net.Conn) error {
	if c.onRequest == nil {
		c.onRequest = OnRequest
	}

	// First header should be a RequestHeader
	// if this is not a RequestHeader
	// they can try to send a new header until conn is closed
	r, err := c.ReadRequest(conn)
	if err != nil {
		return err
	}

	ok := c.onRequest(r)
	if !ok {
		_, err = conn.Write(NotOkResponseHeaderBytes)
		return err
	}

	_, err = conn.Write(OkResponseHeaderBytes)
	if err != nil {
		return err
	}

	// Proceeding bytes will be a chain of EncodedFileHeader and actual file bytes
	// if EncodedEndHeader is received, conn will be closed
	// we will pass the reader to Receiver, which will handle all the parsing for files
	return c.receiver.receive(conn)
}

func (c *Client) ReadRequest(conn net.Conn) (*RequestHeader, error) {
	reader := bufio.NewReader(conn)

	for {
		header, err := reader.ReadString(delim)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, err
			}
			continue
		}

		h := EncodedRequestHeader(header)

		hd, err := h.Parse()
		if err != nil {
			return nil, err
		}

		parsed, ok := hd.(*RequestHeader)
		if !ok {
			return nil, errors.New("failed to type assert to RequestHeader")
		}

		return parsed, nil
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

func OnRequest(r *RequestHeader) bool {
	confirm := false

	title := fmt.Sprintf("Accept %d files? (%d bytes) \n", r.n, r.nbytes)

	huh.NewConfirm().
		Title(title).
		Affirmative("Yes").
		Negative("No").
		Value(&confirm).
		Run()

	return confirm
}

func OnNewPeer(id, fingerprint string) bool {
	confirm := false

	title := warningStyle.Render("The authenticity of host:%s can't be established. Are you sure you want to continue connecting? (Yes/No)")

	huh.NewConfirm().
		Title(title).
		Affirmative("Yes").
		Negative("No").
		Value(&confirm).
		Run()

	return confirm
}
