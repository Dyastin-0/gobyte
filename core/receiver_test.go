package core

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReceive(t *testing.T) {
	tempDir := t.TempDir()
	r := NewReceiver(tempDir)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	go func() {
		err := r.receive(serverConn, &RequestHeader{n: 1})
		if err != nil {
			t.Logf("receive error: %v", err)
		}
	}()

	h := &FileHeader{
		size: 4,
		path: "test",
		name: "test.txt",
	}

	hEncoded, err := h.Encoded()
	if err != nil {
		t.Error(err)
	}

	encodedBytes, ok := hEncoded.(*EncodedFileHeader)
	if !ok {
		t.Error("failed to type assert to *EncodedFileHeader")
	}

	bytesfull := append(*encodedBytes, []byte("test")...)
	bytesfull = append(bytesfull, EndHeaderBytes...)

	_, err = clientConn.Write(bytesfull)
	if err != nil {
		t.Error(err)
	}

	time.Sleep(time.Millisecond * 50)

	path := filepath.Join(tempDir, h.path, h.name)
	_, err = os.Stat(path)
	if err != nil {
		t.Error(err)
	}
}

func TestReceiveRecoverCorrupt(t *testing.T) {
	tempDir := t.TempDir()
	r := NewReceiver(tempDir)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	go func() {
		err := r.receive(serverConn, &RequestHeader{n: 1})
		if err != nil {
			t.Logf("receive error: %v", err)
		}
	}()

	b := []byte("magic bytes")
	b = append(b, []byte{headerDelim, headerDelim, headerDelim, delim}...)
	corruptedHeader := EncodedFileHeader(b)

	_, err := clientConn.Write(corruptedHeader)
	if err != nil {
		t.Error(err)
	}

	h := &FileHeader{
		size: 4,
		path: "test",
		name: "test.txt",
	}

	hEncoded, err := h.Encoded()
	if err != nil {
		t.Error(err)
	}

	encodedBytes, ok := hEncoded.(*EncodedFileHeader)
	if !ok {
		t.Error("failed to type assert to *EncodedFileHeader")
	}

	bytesfull := append(*encodedBytes, []byte("test")...)
	bytesfull = append(bytesfull, EndHeaderBytes...)

	_, err = clientConn.Write(bytesfull)
	if err != nil {
		t.Error(err)
	}

	time.Sleep(time.Millisecond * 50)

	path := filepath.Join(tempDir, h.path, h.name)
	_, err = os.Stat(path)
	if err != nil {
		t.Error(err)
	}
}

func TestDefaultWriteFunc(t *testing.T) {
	tempDir := t.TempDir()
	r := NewReceiver(tempDir)

	h := &FileHeader{
		path: "docs",
		name: "readme.txt",
		size: 6,
	}

	rd := strings.NewReader("readme")
	n, err := r.Write(rd, h, &RequestHeader{n: 1}, 1)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, h.size, n)

	path := filepath.Join(tempDir, h.path, h.name)
	_, err = os.Stat(path)
	if err != nil {
		t.Error(err)
	}

	rd = strings.NewReader("readme")
	n, err = r.Write(rd, h, &RequestHeader{n: 1}, 1)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, h.size, n)

	path = filepath.Join(tempDir, h.path, "readme (1).txt")
	_, err = os.Stat(path)
	if err != nil {
		t.Error(err)
	}
}
