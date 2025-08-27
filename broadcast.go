package gobyte

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

const (
	TypeBroadcastMessageError = "error"
	TypeBroadcastMessageHello = "hello"
)

var (
	ErrMalformedBroadcastMessage = errors.New("malformed broadcast message")

	MalformedBroadcastMessage = &BroadcastMessage{
		Type: TypeBroadcastMessageError,
		Data: "Malformed message",
	}

	HelloInterval = time.Second * 2
)

var EncodedHelloBroadcastMessage = func(message any) *EncodedUDPMessage {
	hello := &BroadcastMessage{
		Type: TypeBroadcastMessageHello,
		Data: fmt.Sprintf("%v", message),
		Name: hex.EncodeToString([]byte(hostname())),
	}
	encoded, err := hello.Encoded()
	if err != nil {
		panic(err)
	}

	encodedBytes, ok := encoded.(*EncodedUDPMessage)
	if !ok {
		panic("failed to type assert to EncodedUDPMessage")
	}

	return encodedBytes
}

var EncodedMalformedBroadcastMessage = func() *EncodedUDPMessage {
	encoded, err := MalformedBroadcastMessage.Encoded()
	if err != nil {
		panic(err)
	}

	encodedBytes, ok := encoded.(*EncodedUDPMessage)
	if !ok {
		panic("failed to type assert to EncodedUDPMessage")
	}

	return encodedBytes
}

type BroadcastMessage struct {
	Type string
	Data string
	Name string
}

type peer struct {
	name      string
	data      any
	addr      *net.UDPAddr
	lastHello time.Time
}

type EncodedUDPMessage []byte

type in struct {
	bytes *EncodedUDPMessage
	addr  *net.UDPAddr
}

type out struct {
	bytes *EncodedUDPMessage
	addr  *net.UDPAddr
}

type Broadcaster struct {
	addr    string
	ln      *net.UDPConn
	inch    chan *in
	outch   chan *out
	message any

	mu    sync.Mutex
	peers map[string]*peer
}

func NewBroadcaster(addr string, message any) *Broadcaster {
	return &Broadcaster{
		addr:    addr,
		inch:    make(chan *in, 100),
		outch:   make(chan *out, 100),
		peers:   make(map[string]*peer),
		message: message,
	}
}

func (e *EncodedUDPMessage) String() string {
	return string(*e)
}

func (e *EncodedUDPMessage) Parse() (Header, error) {
	parts := strings.Split(e.String(), string(headerDelim))
	if len(parts) != 3 {
		return MalformedBroadcastMessage, ErrMalformedBroadcastMessage
	}

	t := parts[0]
	t = strings.TrimSpace(t)
	if !validType(t) {
		return MalformedBroadcastMessage, ErrMalformedBroadcastMessage
	}

	d := parts[1]

	n := parts[2]
	n = strings.TrimSpace(n)
	if n == "" {
		return MalformedBroadcastMessage, ErrMalformedBroadcastMessage
	}

	return &BroadcastMessage{Type: t, Data: d, Name: n}, nil
}

func (bm *BroadcastMessage) Encoded() (Encoded, error) {
	str := fmt.Sprintf("%s%s%s%s%s", bm.Type, string(headerDelim), bm.Data, string(headerDelim), bm.Name)

	b := EncodedUDPMessage(str)
	return &b, nil
}

func (b *Broadcaster) write(out *out) (n int, err error) {
	n, err = b.ln.WriteToUDP(*out.bytes, out.addr)
	if err != nil {
		log.Printf("[err] %v\n", err)
	}
	return
}

func (b *Broadcaster) Listen(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", b.addr)
	if err != nil {
		return err
	}

	ln, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	err = ln.SetWriteBuffer(1024 * 1024)
	if err != nil {
		return err
	}

	file, err := ln.File()
	if err != nil {
		return err
	}
	defer file.Close()

	// Set broadcast socket permission
	syscall.SetsockoptInt(int(file.Fd()), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)

	b.ln = ln

	go b.b(ctx)
	go b.listenBytes(ctx)
	go b.helloer(ctx)

	_, port, err := net.SplitHostPort(b.addr)
	if err != nil {
		return err
	}

	intPort, err := strconv.Atoi(port)
	if err != nil {
		return err
	}

	for {
		buf := make([]byte, 1024)

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			n, remoteAddr, err := ln.ReadFromUDP(buf)
			if err != nil {
				log.Printf("[err] %v\n", err)
				continue
			}

			// Ignore broadcasts from self
			if remoteAddr.IP.Equal(outboundIP()) && remoteAddr.Port == intPort {
				continue
			}

			encodedBytes := EncodedUDPMessage(buf[:n])
			b.inch <- &in{bytes: &encodedBytes, addr: remoteAddr}
		}
	}
}

func (b *Broadcaster) listenBytes(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out := <-b.outch:
			go b.write(out)
		case in := <-b.inch:
			// Ignore err, since Parse will always return a BroadcastMessage
			msg, _ := in.bytes.Parse()

			parsedMessage, ok := msg.(*BroadcastMessage)
			if !ok {
				continue
			}

			switch parsedMessage.Type {
			case TypeBroadcastMessageError:
				log.Printf("[err] %v", ErrMalformedBroadcastMessage)
				go b.write(&out{bytes: EncodedMalformedBroadcastMessage(), addr: in.addr})
			case TypeBroadcastMessageHello:
				hnbytes, err := hex.DecodeString(parsedMessage.Name)
				if err != nil {
					log.Printf("[err] failed to decode string: %s", parsedMessage.Data)
					continue
				}

				hn := string(hnbytes)

				b.mu.Lock()
				if _, ok := b.peers[hn]; !ok {
					b.peers[hn] = &peer{
						addr:      in.addr,
						lastHello: time.Now(),
						data:      parsedMessage.Data,
						name:      hn,
					}
				} else {
					b.peers[hn].lastHello = time.Now()
				}
				b.mu.Unlock()

				// Immediately send a hello back, faster discovery
				go b.write(&out{bytes: EncodedHelloBroadcastMessage(b.message), addr: in.addr})
			}
		}
	}
}

func (b *Broadcaster) b(ctx context.Context) error {
	ticker := time.NewTicker(time.Second * 15)
	_, port, err := net.SplitHostPort(b.addr)
	if err != nil {
		return err
	}
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("255.255.255.255:%s", port))
	if err != nil {
		return err
	}

	go b.write(&out{bytes: EncodedHelloBroadcastMessage(b.message), addr: addr})

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			go b.write(&out{bytes: EncodedHelloBroadcastMessage(b.message), addr: addr})
		}
	}
}

func (b *Broadcaster) helloer(ctx context.Context) error {
	ticker := time.NewTicker(HelloInterval)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			b.mu.Lock()
			for _, peer := range b.peers {
				elapsed := time.Since(peer.lastHello)

				if elapsed > HelloInterval+3*time.Second {
					delete(b.peers, peer.name)
					continue
				}

				go b.write(&out{bytes: EncodedHelloBroadcastMessage(b.message), addr: peer.addr})
			}
			b.mu.Unlock()
		}
	}
}

func validType(t string) (b bool) {
	switch t {
	case TypeBroadcastMessageHello:
		fallthrough
	case TypeBroadcastMessageError:
		b = true
	default:
		b = false
	}

	return
}

func hostname() string {
	hn, err := os.Hostname()
	if err != nil {
		hn = fmt.Sprintf("%s-%s", "unknown", uuid.NewString())
	}

	return hn
}

func outboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}
