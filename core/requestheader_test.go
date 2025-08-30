package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRequestHeader(t *testing.T) {
	h := &RequestHeader{
		n:       10,
		nbytes:  124,
		version: VERSION,
	}

	hEncoded, err := h.Encoded()
	if err != nil {
		t.Error(err)
	}

	hEncodedBytes, ok := hEncoded.(*EncodedRequestHeader)
	if !ok {
		t.Error("failed to type assert to EncodedRequestHeader")
	}

	hh, err := hEncodedBytes.Parse()
	if err != nil {
		t.Error(err)
	}

	hhEncoded, err := hh.Encoded()
	if err != nil {
		t.Error(err)
	}

	hhEncodedBytes, ok := hhEncoded.(*EncodedRequestHeader)
	if !ok {
		t.Error("failed to type assert to EncodedRequestHeader")
	}

	expected := EncodedRequestHeader(
		fmt.Appendf(nil,
			"%s%s%s%d%s%f%s%s",
			string(headerDelim),
			VERSION,
			string(headerDelim),
			h.n,
			string(headerDelim),
			h.nbytes,
			string(headerDelim),
			string(delim),
		),
	)

	assert.Equal(t, *hEncodedBytes, *hhEncodedBytes)
	assert.Equal(t, expected, *hEncodedBytes)
	assert.Equal(t, expected, *hhEncodedBytes)
}

func TestParseMismatchRequestHeader(t *testing.T) {
	r := &RequestHeader{
		nbytes:  420,
		n:       69,
		version: "0.1",
	}

	_, err := r.Encoded()
	if err != ErrVersionMismatch {
		t.Errorf("expected ErrVersionMismatch, but got %v\n", err)
	}

	encodedBytes := EncodedRequestHeader(
		fmt.Appendf(nil,
			"%s%s%s%s%s%s%s%s",
			string(headerDelim),
			"0.1",
			string(headerDelim),
			"69",
			string(headerDelim),
			"420",
			string(headerDelim),
			string(delim),
		),
	)

	_, err = encodedBytes.Parse()
	if err != ErrVersionMismatch {
		t.Errorf("expected ErrVersionMismatch, but got %v\n", err)
	}
}
