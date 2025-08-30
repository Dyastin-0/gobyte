package core

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrMalformedSummaryHeader = errors.New("malformed summary header")

type SummaryHeader struct {
	nBytes float64
	files  []*FileHeader

	nFailedBytes float64
	failedFiles  []*FileHeader
}

type EncodedSummaryHeader []byte

// Encoded creates 0x1F<nBytes>0x1F<nFailedBytes>0x1F0x1D
func (s *SummaryHeader) Encoded() (Encoded, error) {
	if s.nBytes < 0 {
		return nil, ErrMalformedSummaryHeader
	}

	if s.nFailedBytes < 0 {
		return nil, ErrMalformedSummaryHeader
	}

	h := EncodedSummaryHeader(
		fmt.Appendf(nil,
			"%s%f%s%f%s%s",
			string(headerDelim),
			s.nBytes,
			string(headerDelim),
			s.nFailedBytes,
			string(headerDelim),
			string(delim),
		),
	)

	return &h, nil
}

func (e *EncodedSummaryHeader) String() string {
	return string(*e)
}

func (e *EncodedSummaryHeader) Parse() (Header, error) {
	parts := strings.Split(e.String(), string(headerDelim))
	if len(parts) != 4 {
		return nil, ErrMalformedSummaryHeader
	}

	h := &SummaryHeader{}

	n := parts[1]
	n = strings.TrimSpace(n)
	parsedN, err := strconv.ParseFloat(n, 64)
	if err != nil {
		return nil, err
	}

	h.nBytes = parsedN

	nFailed := parts[2]
	nFailed = strings.TrimSpace(nFailed)
	parsedNFailed, err := strconv.ParseFloat(nFailed, 64)
	if err != nil {
		return nil, err
	}

	h.nFailedBytes = parsedNFailed

	return h, nil
}
