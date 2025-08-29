package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFileHeader(t *testing.T) {
	h := &FileHeader{
		size: int64(64),
		name: "test.txt",
		path: "./",
	}

	hEncoded, err := h.Encoded()
	if err != nil {
		t.Error(err)
	}

	hEncodedBytes, ok := hEncoded.(*EncodedFileHeader)
	if !ok {
		t.Error("failed to type assert to EncodedFileHeader")
	}

	hh, err := hEncodedBytes.Parse()
	if err != nil {
		t.Error(err)
	}

	hhEncoded, err := hh.Encoded()
	if err != nil {
		t.Error(err)
	}

	hhEncodedBytes, ok := hhEncoded.(*EncodedFileHeader)
	if !ok {
		t.Error("failed to type assert to EncodedFileHeader")
	}

	expected := EncodedFileHeader(fmt.Appendf(nil,
		"%s%d%s%s%s%s%s%s",
		string(headerDelim),
		h.size,
		string(headerDelim),
		h.name,
		string(headerDelim),
		h.path,
		string(headerDelim),
		string(delim),
	))

	assert.Equal(t, *hEncodedBytes, *hhEncodedBytes)
	assert.Equal(t, expected, *hEncodedBytes)
	assert.Equal(t, expected, *hhEncodedBytes)

	// Append a magic bytes
	hEncodedBytes, ok = hhEncoded.(*EncodedFileHeader)
	if !ok {
		t.Error("failed to type assert to EncodedFileHeader")
	}
	encodedHeader := EncodedFileHeader(append([]byte("Magic Bytes"), *hEncodedBytes...))

	hhh, err := encodedHeader.Parse()
	if err != nil {
		t.Error(err)
	}

	hhhEncoded, err := hhh.Encoded()
	if err != nil {
		t.Error(err)
	}

	hhhEncodedBytes, ok := hhhEncoded.(*EncodedFileHeader)
	if !ok {
		t.Error("failed to type assert to EncodedFileHeader")
	}

	assert.Equal(t, *hEncodedBytes, *hhhEncodedBytes)
	assert.Equal(t, expected, *hhhEncodedBytes)
}
