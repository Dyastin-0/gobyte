package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseResponseHeader(t *testing.T) {
	h := &ResponseHeader{
		ok: ResponseOk,
	}

	hEncoded, err := h.Encoded()
	if err != nil {
		t.Error(err)
	}

	hEncodedBytes, ok := hEncoded.(*EncodedResponseHeader)
	if !ok {
		t.Error("failed to type assert to EncodedResponseHeader")
	}

	hh, err := hEncodedBytes.Parse()
	if err != nil {
		t.Error(err)
	}

	hhEncoded, err := hh.Encoded()
	if err != nil {
		t.Error(err)
	}

	hhEncodedBytes, ok := hhEncoded.(*EncodedResponseHeader)
	if !ok {
		t.Error("failed to type assert to EncodedResponseHeader")
	}

	expected := EncodedResponseHeader(OkResponseHeaderBytes)
	assert.Equal(t, *hEncodedBytes, *hhEncodedBytes)
	assert.Equal(t, expected, *hEncodedBytes)
	assert.Equal(t, expected, *hhEncodedBytes)
}
