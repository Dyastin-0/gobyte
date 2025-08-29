package core

import (
	"context"
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

	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		t.Error(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go r.Listen(ctx, ln)

	h := &FileHeader{
		size: 4,
		path: "test",
		name: "test.txt",
	}

	hEncoded, err := h.Encoded()
	if err != nil {
		t.Error(err)
	}

	hEncoded = hEncoded.(*EncodedFileHeader)

	encodedBytes, ok := hEncoded.(*EncodedFileHeader)
	if !ok {
		t.Error("failed to type assert to *EncodedFileHeader")
	}

	bytesfull := append(*encodedBytes, []byte("test")...)
	bytesfull = append(bytesfull, EndHeaderBytes...)

	conn, err := net.Dial("tcp", ":8080")
	if err != nil {
		t.Error(err)
	}

	_, err = conn.Write(bytesfull)
	if err != nil {
		t.Error(err)
	}

	time.Sleep(time.Millisecond * 50)

	path := filepath.Join(tempDir, h.path, h.name)
	_, err = os.Stat(path)
	if err != nil {
		t.Error(err)
	}

	cancel()
	time.Sleep(time.Millisecond * 50)
}

func TestReceiveRecoverCorrupt(t *testing.T) {
	tempDir := t.TempDir()
	r := NewReceiver(tempDir)
	ctx, cancel := context.WithCancel(context.Background())

	ln, err := net.Listen("tcp", ":9090")
	if err != nil {
		t.Error(err)
	}

	go r.Listen(ctx, ln)

	b := []byte("magic bytes")
	b = append(b, []byte{headerDelim, headerDelim, headerDelim, delim}...)
	corruptedHeader := EncodedFileHeader(b)

	conn, err := net.Dial("tcp", ":9090")
	if err != nil {
		t.Error(err)
	}

	_, err = conn.Write(corruptedHeader)
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
	bytesfull = append(*encodedBytes, EndHeaderBytes...)

	_, err = conn.Write(bytesfull)
	if err != nil {
		t.Error(err)
	}

	time.Sleep(time.Millisecond * 50)

	path := filepath.Join(tempDir, h.path, h.name)
	_, err = os.Stat(path)
	if err != nil {
		t.Error(err)
	}

	cancel()
	time.Sleep(time.Millisecond * 5)
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
	n, err := r.Write(rd, h)
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
	n, err = r.Write(rd, h)
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
