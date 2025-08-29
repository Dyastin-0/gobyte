// Package gobyte ...
package gobyte

const (
	delim       = byte(0x1D)
	headerDelim = byte(0x1F)
	endDelim    = byte(0x1E)
)

type Header interface {
	Encoded() (Encoded, error)
}

type Encoded interface {
	String() string
	Parse() (Header, error)
}
