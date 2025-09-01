package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
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
	HelloInterval                = time.Second * 2
	validTypes                   = map[string]bool{
		TypeBroadcastMessageHello: true,
		TypeBroadcastMessageError: true,
	}
)

type BroadcastMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Name string `json:"name"`
}

type peer struct {
	name      string
	data      string
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
	addr                string
	ln                  *net.UDPConn
	inch                chan *in
	outch               chan *out
	message             any
	encodedHelloMsg     *EncodedUDPMessage
	encodedMalformedMsg *EncodedUDPMessage
	receiveOnly         bool

	mu    sync.Mutex
	peers map[string]*peer
}

func NewBroadcaster(addr string, message any) *Broadcaster {
	b := &Broadcaster{
		addr:        addr,
		inch:        make(chan *in, 100),
		outch:       make(chan *out, 100),
		peers:       make(map[string]*peer),
		message:     message,
		receiveOnly: false,
	}

	b.encodedHelloMsg = b.createHelloBroadcastMessage(message)
	b.encodedMalformedMsg = b.createMalformedBroadcastMessage()

	return b
}

func NewReceiveOnlyBroadcaster(addr string) *Broadcaster {
	b := &Broadcaster{
		addr:        addr,
		inch:        make(chan *in, 100),
		outch:       make(chan *out, 100),
		peers:       make(map[string]*peer),
		receiveOnly: true,
	}

	return b
}

func (b *Broadcaster) createHelloBroadcastMessage(message any) *EncodedUDPMessage {
	hello := &BroadcastMessage{
		Type: TypeBroadcastMessageHello,
		Data: fmt.Sprintf("%v", message),
		Name: hostname(),
	}
	encoded, err := hello.Encoded()
	if err != nil {
		panic(err)
	}
	return encoded
}

func (b *Broadcaster) createMalformedBroadcastMessage() *EncodedUDPMessage {
	malformed := &BroadcastMessage{
		Type: TypeBroadcastMessageError,
		Data: "Malformed message",
		Name: hostname(),
	}
	encoded, err := malformed.Encoded()
	if err != nil {
		panic(err)
	}
	return encoded
}

func (e *EncodedUDPMessage) String() string {
	return string(*e)
}

func (e *EncodedUDPMessage) Parse() (*BroadcastMessage, error) {
	var bm BroadcastMessage
	err := json.Unmarshal(*e, &bm)
	if err != nil {
		return &BroadcastMessage{
			Type: TypeBroadcastMessageError,
			Data: "Malformed message",
			Name: hostname(),
		}, ErrMalformedBroadcastMessage
	}

	if !validTypes[bm.Type] {
		return &BroadcastMessage{
			Type: TypeBroadcastMessageError,
			Data: "Invalid message type",
			Name: hostname(),
		}, ErrMalformedBroadcastMessage
	}

	if bm.Name == "" {
		return &BroadcastMessage{
			Type: TypeBroadcastMessageError,
			Data: "Missing name field",
			Name: hostname(),
		}, ErrMalformedBroadcastMessage
	}

	return &bm, nil
}

func (bm *BroadcastMessage) Encoded() (*EncodedUDPMessage, error) {
	jsonBytes, err := json.Marshal(bm)
	if err != nil {
		return nil, err
	}

	encoded := EncodedUDPMessage(jsonBytes)
	return &encoded, nil
}

func (b *Broadcaster) write(out *out) (n int, err error) {
	n, err = b.ln.WriteToUDP(*out.bytes, out.addr)
	if err != nil {
		log.Printf("[err] %v\n", err)
	}
	return
}

func (b *Broadcaster) Init() error {
	addr, err := net.ResolveUDPAddr("udp", b.addr)
	if err != nil {
		return err
	}

	ln, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	if !b.receiveOnly {
		err = ln.SetWriteBuffer(1024 * 1024)
		if err != nil {
			ln.Close()
			return err
		}

		file, err := ln.File()
		if err != nil {
			ln.Close()
			return err
		}
		defer file.Close()

		err = syscall.SetsockoptInt(int(file.Fd()), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
		if err != nil {
			ln.Close()
			return err
		}
	}

	b.ln = ln
	return nil
}

func (b *Broadcaster) Close() error {
	if b.ln != nil {
		return b.ln.Close()
	}
	return nil
}

func (b *Broadcaster) GetPeers() map[string]*peer {
	b.mu.Lock()
	defer b.mu.Unlock()

	peers := make(map[string]*peer)
	for k, v := range b.peers {
		peers[k] = &peer{
			name:      v.name,
			data:      v.data,
			addr:      v.addr,
			lastHello: v.lastHello,
		}
	}
	return peers
}

func (b *Broadcaster) Start(ctx context.Context) error {
	err := b.Init()
	if err != nil {
		return err
	}
	defer b.Close()

	if !b.receiveOnly {
		go b.b(ctx)
	}

	go b.listenBytes(ctx)

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
			n, remoteAddr, err := b.ln.ReadFromUDP(buf)
			if err != nil {
				log.Printf("[err] %v\n", err)
				continue
			}

			if !b.receiveOnly && remoteAddr.IP.Equal(outboundIP()) && remoteAddr.Port == intPort {
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
			if !b.receiveOnly {
				go b.write(out)
			}
		case in := <-b.inch:
			msg, err := in.bytes.Parse()

			switch msg.Type {
			case TypeBroadcastMessageError:
				if err != nil {
					log.Printf("[err] %v", err)
				}
				if !b.receiveOnly {
					go b.write(&out{bytes: b.encodedMalformedMsg, addr: in.addr})
				}
			case TypeBroadcastMessageHello:
				hn := msg.Name

				b.mu.Lock()
				if _, ok := b.peers[hn]; !ok {
					b.peers[hn] = &peer{
						addr:      in.addr,
						lastHello: time.Now(),
						data:      msg.Data,
						name:      hn,
					}
				} else {
					b.peers[hn].lastHello = time.Now()
				}
				b.mu.Unlock()
			}
		}
	}
}

func (b *Broadcaster) b(ctx context.Context) error {
	ticker := time.NewTicker(HelloInterval)
	defer ticker.Stop()

	_, port, err := net.SplitHostPort(b.addr)
	if err != nil {
		return err
	}

	broadcastAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("255.255.255.255:%s", port))
	if err != nil {
		return err
	}

	go b.write(&out{bytes: b.encodedHelloMsg, addr: broadcastAddr})

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			go b.write(&out{bytes: b.encodedHelloMsg, addr: broadcastAddr})

			b.mu.Lock()
			for name, peer := range b.peers {
				if time.Since(peer.lastHello) > HelloInterval+2*time.Second {
					delete(b.peers, name)
				}
			}
			b.mu.Unlock()
		}
	}
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
