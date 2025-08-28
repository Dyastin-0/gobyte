package gobyte

import (
	"io"
	"net"
	"os"
	"path/filepath"
)

type Sender struct {
	fileselector *FileSelector
}

type summary struct {
	nBytes int64
	files  []*FileHeader

	nFailedBytes int64
	failedFiles  []*FileHeader
}

func NewSender(dir string) *Sender {
	return &Sender{
		fileselector: NewFileSelector(dir),
	}
}

func (s *Sender) Send(conn net.Conn, files []*FileHeader) (*summary, error) {
	summ := &summary{}

	return summ, nil
}

func (s *Sender) Open(f *FileHeader) (io.Reader, error) {
	path := filepath.Join(f.path, f.name)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (s *Sender) WriteHeader(conn net.Conn, f *FileHeader) error {
	return nil
}

func (s *Sender) Write(conn net.Conn, file io.Reader) (int64, error) {
	return 0, nil
}
