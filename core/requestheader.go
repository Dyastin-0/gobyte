package core

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrMalformedRequestHeader = errors.New("malformed request header")

type RequestHeader struct {
	n      int
	nbytes int64
}

type EncodedRequestHeader []byte

func (e *EncodedRequestHeader) String() string {
	return string(*e)
}

func (e *EncodedRequestHeader) Parse() (Header, error) {
	parts := strings.Split(e.String(), string(headerDelim))
	if len(parts) != 4 {
		return nil, ErrMalformedRequestHeader
	}

	n := parts[1]
	n = strings.TrimSpace(n)
	parsedN, err := strconv.Atoi(n)
	if err != nil {
		return nil, err
	}

	if parsedN <= 0 {
		return nil, ErrMalformedRequestHeader
	}

	nbytes := parts[2]
	nbytes = strings.TrimSpace(nbytes)
	parsedNbytes, err := strconv.ParseInt(nbytes, 10, 64)
	if err != nil {
		return nil, err
	}

	if parsedNbytes < 0 {
		return nil, ErrMalformedRequestHeader
	}

	return &RequestHeader{n: parsedN, nbytes: parsedNbytes}, nil
}

func (r *RequestHeader) Encoded() (Encoded, error) {
	if r.n <= 0 {
		return nil, ErrMalformedRequestHeader
	}

	if r.n < 0 {
		return nil, ErrMalformedRequestHeader
	}

	hd := fmt.Sprintf(
		"%s%d%s%d%s%s",
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
