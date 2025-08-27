package gobyte

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHeader(t *testing.T) {
	h := &FileHeader{
		size: int64(64),
		name: "test.txt",
		path: "./",
	}

	hEncoded, err := h.Encoded()
	if err != nil {
		t.Error(err)
	}

	encodedBytes, ok := hEncoded.(*EncodedHeader)
	if !ok {
		t.Error("failed to type assert to EncodedHeader")
	}

	hh, err := encodedBytes.Parse()
	if err != nil {
		t.Error(err)
	}

	hhEncoded, err := hh.Encoded()
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, hEncoded, hhEncoded)

	// Append a magic bytes
	encodedBytes, ok = hhEncoded.(*EncodedHeader)
	if !ok {
		t.Error("failed to type assert to *EncodedHeader")
	}
	encodedHeader := EncodedHeader(append([]byte("Magic Bytes"), *encodedBytes...))

	hhh, err := encodedHeader.Parse()
	if err != nil {
		t.Error(err)
	}

	hhhEncoded, err := hhh.Encoded()
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, hEncoded, hhhEncoded)
}

func TestFileHeaderEncoded(t *testing.T) {
	h := &FileHeader{
		size: int64(64),
		name: "test.txt",
		path: "./",
	}

	hEncoded, err := h.Encoded()
	if err != nil {
		t.Error(err)
	}

	encodedBytes, ok := hEncoded.(*EncodedHeader)
	if !ok {
		t.Error("failed to type assert to EncodedHeader")
	}

	expected := fmt.Appendf(nil,
		"%s%d%s%s%s%s%s%s",
		string(headerDelim),
		h.size,
		string(headerDelim),
		h.name,
		string(headerDelim),
		h.path,
		string(headerDelim),
		string(delim),
	)

	assert.Equal(t, *encodedBytes, EncodedHeader(expected))
}

func TestEndHeader(t *testing.T) {
	b := []byte("Magic Bytes")
	b = append(b, EndHeaderBytes...)
	h := EncodedEndHeader(b)

	endHeader, err := h.Parse()
	if err != nil {
		t.Error(err)
	}

	parsedHeader, ok := endHeader.(*EndHeader)
	if !ok {
		t.Error("failed to type assert to *EndHeader")
	}

	assert.Equal(t, endDelim, parsedHeader.ender)
}
