// Package gobyte ...
package gobyte

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	delim       = byte(0x1D)
	headerDelim = byte(0x1F)
	endDelim    = byte(0x1E)
)

var (
	ErrMalformedFileHeader = errors.New("malformed file header")
	ErrMalformedEndHeader  = errors.New("malformed end header")

	EndHeaderBytes = []byte{headerDelim, endDelim, headerDelim, delim}
)

type Header interface {
	Encoded() (Encoded, error)
}

type Encoded interface {
	String() string
	Parse() (Header, error)
}

type (
	EncodedHeader    []byte
	EncodedEndHeader []byte
)

type FileHeader struct {
	name string
	path string
	size int64
}

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

func (e *EncodedHeader) String() string {
	return string(*e)
}

// Parse will try to decode an EncodedHeader to a FileHeader
// which splits to parts: ["", "size", "name", "path", "delim"]
// where parts[0] can either be an empty string or garbage
func (e *EncodedHeader) Parse() (Header, error) {
	parts := strings.Split(e.String(), string(headerDelim))
	if len(parts) != 5 {
		return nil, ErrMalformedFileHeader
	}

	size, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, err
	}
	if size < 0 {
		return nil, ErrMalformedFileHeader
	}

	name := strings.TrimSpace(parts[2])
	if name == "" {
		return nil, ErrMalformedFileHeader
	}

	path := strings.TrimSpace(parts[3])
	if path == "" {
		return nil, ErrMalformedFileHeader
	}

	return &FileHeader{
		name: name,
		path: path,
		size: size,
	}, nil
}

// Encoded creates 0x1Fsize0x1Fname0x1Fpath0x1F0x1D
func (h *FileHeader) Encoded() (Encoded, error) {
	if h.size < 0 {
		return nil, ErrMalformedFileHeader
	}

	h.name = strings.TrimSpace(h.name)

	if h.name == "" {
		return nil, ErrMalformedFileHeader
	}

	h.path = strings.TrimSpace(h.path)
	if h.path == "" {
		return nil, ErrMalformedFileHeader
	}

	str := []string{strconv.FormatInt(h.size, 10), h.name, h.path}
	hd := strings.Join(str, string(headerDelim))
	hd = fmt.Sprintf("%s%s%s%s", string(headerDelim), hd, string(headerDelim), string(delim))

	encoded := EncodedHeader(hd)
	return &encoded, nil
}
