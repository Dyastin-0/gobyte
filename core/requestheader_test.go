package core

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRequestHeader(t *testing.T) {
	h := &RequestHeader{
		n:      10,
		nbytes: 124,
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
			"%s%s%s%s%s%s",
			string(headerDelim),
			strconv.FormatInt(int64(h.n), 10),
			string(headerDelim),
			strconv.FormatInt(h.nbytes, 10),
			string(headerDelim),
			string(delim),
		),
	)

	assert.Equal(t, *hEncodedBytes, *hhEncodedBytes)
	assert.Equal(t, expected, *hEncodedBytes)
	assert.Equal(t, expected, *hhEncodedBytes)
}
