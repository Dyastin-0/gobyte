package gobyte

import (
	"io"
	"net"
	"os"
	"path/filepath"
)

type Sender struct{}

type summary struct {
	nBytes int64
	files  []*file

	nFailedBytes int64
	failedFiles  []*file
}

func NewSender() *Sender {
	return &Sender{}
}

func (s *Sender) Request(conn net.Conn, n int) error {
	return nil
}

func (s *Sender) Send(conn net.Conn, files []*file) (*summary, error) {
	summ := &summary{}

	return summ, nil
}

func (s *Sender) Open(f *file) (io.Reader, error) {
	path := filepath.Join(f.path, f.name)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (s *Sender) WriteHeader(conn net.Conn, h EncodedHeader) error {
	return nil
}

func (s *Sender) Write(conn net.Conn, file io.Reader) (int64, error) {
	return 0, nil
}
