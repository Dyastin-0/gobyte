package core

import (
	"context"
	"encoding/hex"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBroadcastServer(t *testing.T) {
	b := NewBroadcaster(":8080", ":42069")
	ctx, cancel := context.WithCancel(context.Background())
	go b.Start(ctx)

	time.Sleep(time.Millisecond * 50)

	addr, err := net.ResolveUDPAddr("udp", b.addr)
	if err != nil {
		t.Error(err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Error(err)
	}

	msg := &BroadcastMessage{
		Type: TypeBroadcastMessageHello,
		Data: ":42069",
		Name: string(hex.EncodeToString([]byte("TEST"))),
	}

	encodedHeader, err := msg.Encoded()
	if err != nil {
		t.Error(err)
	}

	encodedBytes, ok := encodedHeader.(*EncodedUDPMessage)
	if !ok {
		t.Error("failed to type assert to EncodedUDPMessage")
	}

	conn.Write(*encodedBytes)

	time.Sleep(time.Second * 4)

	b.mu.Lock()
	if p, ok := b.peers["TEST"]; !ok {
		t.Error("expected peer TEST, but not found")
	} else {
		assert.Equal(t, "TEST", p.name)
		assert.Equal(t, msg.Data, p.data)
	}
	b.mu.Unlock()

	cancel()
	time.Sleep(time.Millisecond * 50)
}

func TestPeerDelete(t *testing.T) {
	b := NewBroadcaster(":8080", ":42069")
	ctx, cancel := context.WithCancel(context.Background())
	go b.Start(ctx)

	addr, err := net.ResolveUDPAddr("udp", b.addr)
	if err != nil {
		t.Error(err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Error(err)
	}

	msg := &BroadcastMessage{
		Type: "hello",
		Data: string(hex.EncodeToString([]byte("TEST"))),
	}

	encodedMessage, err := msg.Encoded()
	if err != nil {
		t.Error(err)
	}

	encodedBytes, ok := encodedMessage.(*EncodedUDPMessage)
	if !ok {
		t.Error("failed to type assert to EncodedUDPMessage")
	}

	conn.Write(*encodedBytes)

	time.Sleep(time.Second * 1)

	b.mu.Lock()
	if _, ok := b.peers["TEST"]; ok {
		t.Error("expected peer TEST to be deleted, but found")
	}
	b.mu.Unlock()

	cancel()
}

func TestMalformedBroadcastMessage(t *testing.T) {
	b := NewBroadcaster(":8082", ":42069")
	ctx, cancel := context.WithCancel(context.Background())
	go b.Start(ctx)

	time.Sleep(time.Millisecond * 50)

	msg := []byte{headerDelim, headerDelim}

	addr, err := net.ResolveUDPAddr("udp", b.addr)
	if err != nil {
		t.Error(err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Error(err)
	}

	_, err = conn.Write(msg)
	if err != nil {
		t.Error(err)
	}

	buf := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Error(err)
	}

	encodedBytes := EncodedUDPMessage(buf[:n])

	h, err := encodedBytes.Parse()
	assert.Equal(t, ErrMalformedBroadcastMessage, err)

	parsedHeader, ok := h.(*BroadcastMessage)
	if !ok {
		t.Error("failed to type assert to BroadcastMessage")
	}

	assert.Equal(t, TypeBroadcastMessageError, parsedHeader.Type)

	cancel()
	time.Sleep(time.Millisecond * 50)
}
