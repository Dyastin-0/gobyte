package core

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var ErrMalformedFileHeader = errors.New("malformed file header")

type FileHeader struct {
	name    string
	path    string
	abspath string
	size    int64
}

type EncodedFileHeader []byte

func (h *FileHeader) Open() (*os.File, error) {
	file, err := os.Open(h.abspath)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// Encoded creates 0x1F<size>0x1F<name>0x1F<path>0x1F0x1D
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

	hd := EncodedFileHeader(
		fmt.Sprintf(
			"%s%d%s%s%s%s%s%s",
			string(headerDelim),
			h.size,
			string(headerDelim),
			h.name,
			string(headerDelim),
			h.path,
			string(headerDelim),
			string(delim),
		),
	)

	return &hd, nil
}

func (e *EncodedFileHeader) String() string {
	return string(*e)
}

// Parse will try to decode an EncodedFileHeader to a FileHeader
// which splits to parts: ["", "size", "name", "path", "delim"]
// where parts[0] can either be an empty string or garbage
func (e *EncodedFileHeader) Parse() (Header, error) {
	parts := strings.Split(e.String(), string(headerDelim))
	if len(parts) != 5 {
		return nil, ErrMalformedFileHeader
	}

	size := parts[1]
	size = strings.TrimSpace(size)
	parsedSize, err := strconv.ParseInt(size, 10, 64)
	if err != nil {
		return nil, err
	}
	if parsedSize < 0 {
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
		size: parsedSize,
	}, nil
}
