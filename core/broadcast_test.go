package core

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBroadcastServer(t *testing.T) {
	b := NewBroadcaster(":8080", ":42069")
	ctx := t.Context()

	go b.Start(ctx)
	time.Sleep(time.Millisecond * 50)

	addr, err := net.ResolveUDPAddr("udp", "localhost:8080")
	require.NoError(t, err)

	conn, err := net.DialUDP("udp", nil, addr)
	require.NoError(t, err)
	defer conn.Close()

	msg := &BroadcastMessage{
		Type: TypeBroadcastMessageHello,
		Data: ":42069",
		Name: "TEST",
	}

	encodedHeader, err := msg.Encoded()
	require.NoError(t, err)

	_, err = conn.Write(*encodedHeader)
	require.NoError(t, err)

	var peer *peer
	var found bool
	for range 10 {
		time.Sleep(time.Millisecond * 50)
		peers := b.GetPeers()
		if p, ok := peers["TEST"]; ok {
			peer = p
			found = true
			break
		}
	}

	require.True(t, found, "expected peer TEST, but not found")
	assert.Equal(t, "TEST", peer.name)
	assert.Equal(t, msg.Data, peer.data)
}

func TestPeerDelete(t *testing.T) {
	b := NewBroadcaster(":8081", ":42069")
	ctx := t.Context()

	go b.Start(ctx)
	time.Sleep(time.Millisecond * 50)

	addr, err := net.ResolveUDPAddr("udp", "localhost:8081")
	require.NoError(t, err)

	conn, err := net.DialUDP("udp", nil, addr)
	require.NoError(t, err)
	defer conn.Close()

	msg := &BroadcastMessage{
		Type: TypeBroadcastMessageHello,
		Data: "TEST",
		Name: "",
	}

	encodedMessage, err := msg.Encoded()
	require.NoError(t, err)

	_, err = conn.Write(*encodedMessage)
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 200)

	peers := b.GetPeers()
	assert.Empty(t, peers, "expected no peers due to validation error")
}

func TestMalformedBroadcastMessage(t *testing.T) {
	malformedJSON := EncodedUDPMessage(`{"invalid":"json"`)
	h, parseErr := malformedJSON.Parse()

	assert.Equal(t, ErrMalformedBroadcastMessage, parseErr)
	assert.Equal(t, TypeBroadcastMessageError, h.Type)
	assert.Equal(t, "Malformed message", h.Data)

	b := NewBroadcaster(":8082", ":42069")
	ctx := t.Context()

	go b.Start(ctx)
	time.Sleep(time.Millisecond * 100)

	addr, err := net.ResolveUDPAddr("udp", "localhost:8082")
	require.NoError(t, err)

	conn, err := net.DialUDP("udp", nil, addr)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write(malformedJSON)
	require.NoError(t, err)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	buf := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buf)
	require.NoError(t, err)

	encodedBytes := EncodedUDPMessage(buf[:n])
	response, parseErr := encodedBytes.Parse()

	assert.NoError(t, parseErr)
	assert.Equal(t, TypeBroadcastMessageError, response.Type)
	assert.Equal(t, "Malformed message", response.Data)
}

func TestInvalidMessageType(t *testing.T) {
	invalidTypeJSON := EncodedUDPMessage(`{"type":"invalid","data":"test","name":"TEST"}`)
	h, parseErr := invalidTypeJSON.Parse()

	assert.Equal(t, ErrMalformedBroadcastMessage, parseErr)
	assert.Equal(t, TypeBroadcastMessageError, h.Type)
	assert.Equal(t, "Invalid message type", h.Data)

	b := NewBroadcaster(":8083", ":42069")
	ctx := t.Context()

	go b.Start(ctx)
	time.Sleep(time.Millisecond * 100)

	addr, err := net.ResolveUDPAddr("udp", "localhost:8083")
	require.NoError(t, err)

	conn, err := net.DialUDP("udp", nil, addr)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write(invalidTypeJSON)
	require.NoError(t, err)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	buf := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buf)
	require.NoError(t, err)

	encodedBytes := EncodedUDPMessage(buf[:n])
	response, parseErr := encodedBytes.Parse()

	assert.NoError(t, parseErr)
	assert.Equal(t, TypeBroadcastMessageError, response.Type)
	assert.Equal(t, "Malformed message", response.Data)
}
