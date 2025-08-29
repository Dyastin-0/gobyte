package core

import (
	"errors"
	"strings"
)

var (
	ErrMalformedEndHeader = errors.New("malformed end header")
	EndHeaderBytes        = []byte{headerDelim, endDelim, headerDelim, delim}
)

type EncodedEndHeader []byte

type EndHeader struct {
	ender byte
}

func (e *EndHeader) Encoded() (Encoded, error) {
	if e.ender != endDelim {
		return nil, ErrMalformedEndHeader
	}

	b := EncodedEndHeader(EndHeaderBytes)
	return &b, nil
}

func (e *EncodedEndHeader) String() string {
	return string(*e)
}

// Parse will try to decode EncodedHeader to EndHeader
// which splits to parts: ["", "ender", "delim"] ender = endDelim + delim
// where parts[0] can either be an empty string or garbage
func (e *EncodedEndHeader) Parse() (Header, error) {
	parts := strings.Split(e.String(), string(headerDelim))
	if len(parts) != 3 {
		return nil, ErrMalformedEndHeader
	}

	ender := parts[1]

	if ender != string(endDelim) {
		return nil, ErrMalformedEndHeader
	}

	return &EndHeader{ender: []byte(ender)[0]}, nil
}
