package gobyte

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseEndHeader(t *testing.T) {
	h := &EndHeader{
		ender: endDelim,
	}

	hEncoded, err := h.Encoded()
	if err != nil {
		t.Error(err)
	}

	hEncodedBytes, ok := hEncoded.(*EncodedEndHeader)
	if !ok {
		t.Error("failed to type assert to EncodedEndHeader")
	}

	hh, err := hEncodedBytes.Parse()
	if err != nil {
		t.Error(err)
	}

	hhEncoded, err := hh.Encoded()
	if err != nil {
		t.Error(err)
	}

	hhEncodedBytes, ok := hhEncoded.(*EncodedEndHeader)
	if !ok {
		t.Error("failed to type assert to EncodedEndHeader")
	}

	assert.Equal(t, hEncodedBytes, hhEncodedBytes)

	expected := EncodedEndHeader(EndHeaderBytes)

	assert.Equal(t, expected, *hEncodedBytes)
	assert.Equal(t, expected, *hhEncodedBytes)
}
