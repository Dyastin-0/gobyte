package core

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	VERSION = "0.2"
)

var (
	ErrMalformedRequestHeader = errors.New("malformed request header")
	ErrVersionMismatch        = errors.New("version mismatch")
)

type RequestHeader struct {
	version string
	n       int
	nbytes  int64
}

type EncodedRequestHeader []byte

func (e *EncodedRequestHeader) String() string {
	return string(*e)
}

func (e *EncodedRequestHeader) Parse() (Header, error) {
	parts := strings.Split(e.String(), string(headerDelim))
	if len(parts) != 5 {
		return nil, ErrMalformedRequestHeader
	}

	v := parts[1]
	v = strings.TrimSpace(v)
	if v != VERSION {
		return nil, ErrVersionMismatch
	}

	n := parts[2]
	n = strings.TrimSpace(n)
	parsedN, err := strconv.Atoi(n)
	if err != nil {
		return nil, err
	}

	if parsedN <= 0 {
		return nil, ErrMalformedRequestHeader
	}

	nbytes := parts[3]
	nbytes = strings.TrimSpace(nbytes)
	parsedNbytes, err := strconv.ParseInt(nbytes, 10, 64)
	if err != nil {
		return nil, err
	}

	if parsedNbytes < 0 {
		return nil, ErrMalformedRequestHeader
	}

	return &RequestHeader{version: VERSION, n: parsedN, nbytes: parsedNbytes}, nil
}

func (r *RequestHeader) Encoded() (Encoded, error) {
	if r.n <= 0 {
		return nil, ErrMalformedRequestHeader
	}

	if r.n < 0 {
		return nil, ErrMalformedRequestHeader
	}

	v := r.version
	v = strings.TrimSpace(v)
	if v != VERSION {
		return nil, ErrVersionMismatch
	}

	hd := fmt.Sprintf(
		"%s%s%s%d%s%d%s%s",
		string(headerDelim),
		r.version,
		string(headerDelim),
		r.n,
		string(headerDelim),
		r.nbytes,
		string(headerDelim),
		string(delim),
	)

	b := EncodedRequestHeader(hd)
	return &b, nil
}
