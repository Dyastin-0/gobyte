package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSummaryHeader(t *testing.T) {
	h := &SummaryHeader{
		nBytes:       420,
		nFailedBytes: 68,
	}

	hEncoded, err := h.Encoded()
	if err != nil {
		t.Error(err)
	}

	hEncodedBytes, ok := hEncoded.(*EncodedSummaryHeader)
	if !ok {
		t.Error("failed to type assert to EncodedSummaryHeader")
	}

	hh, err := hEncodedBytes.Parse()
	if err != nil {
		t.Error(err)
	}

	hhEncoded, err := hh.Encoded()
	if err != nil {
		t.Error(err)
	}

	hhEncodedBytes, ok := hhEncoded.(*EncodedSummaryHeader)
	if !ok {
		t.Error("failed to type assert to EncodedSummaryHeader")
	}

	expected := EncodedSummaryHeader(
		fmt.Appendf(
			nil,
			"%s%f%s%f%s%s",
			string(headerDelim),
			h.nBytes,
			string(headerDelim),
			h.nFailedBytes,
			string(headerDelim),
			string(delim),
		),
	)

	assert.Equal(t, *hEncodedBytes, *hhEncodedBytes)
	assert.Equal(t, expected, *hEncodedBytes)
	assert.Equal(t, expected, *hhEncodedBytes)
}
