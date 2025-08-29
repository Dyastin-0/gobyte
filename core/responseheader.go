package core

import (
	"errors"
	"fmt"
	"strings"
)

const (
	ResponseOk    = byte(0x00)
	ResponseNotOK = byte(0x1A)
)

var (
	ErrMalformedResponseHeader = errors.New("malformed response header")
	OkResponseHeaderBytes      = []byte{headerDelim, ResponseOk, headerDelim, delim}
	NotOkResponseHeaderBytes   = []byte{headerDelim, ResponseNotOK, headerDelim, delim}
)

type EncodedResponseHeader []byte

type ResponseHeader struct {
	ok byte
}

func (e *EncodedResponseHeader) String() string {
	return string(*e)
}

func (e *EncodedResponseHeader) Parse() (Header, error) {
	parts := strings.Split(e.String(), string(headerDelim))
	if len(parts) != 3 {
		return nil, ErrMalformedResponseHeader
	}

	ok := parts[1]
	ok = strings.TrimSpace(ok)
	bytesOk := []byte(ok)[0]
	if bytesOk != ResponseNotOK && bytesOk != ResponseOk {
		return nil, ErrMalformedResponseHeader
	}

	return &ResponseHeader{ok: bytesOk}, nil
}

func (r *ResponseHeader) Encoded() (Encoded, error) {
	if r.ok != ResponseOk && r.ok != ResponseNotOK {
		return nil, ErrMalformedResponseHeader
	}

	hd := fmt.Sprintf(
		"%s%s%s%s",
		string(headerDelim),
		string(r.ok),
		string(headerDelim),
		string(delim),
	)

	b := EncodedResponseHeader(hd)
	return &b, nil
}
