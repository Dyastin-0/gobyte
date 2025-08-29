package gobyte

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
)

type Sender struct{}

type summary struct {
	nBytes int64
	files  []*FileHeader

	nFailedBytes int64
	failedFiles  []*FileHeader
}

func NewSender() *Sender {
	return &Sender{}
}

func (s *Sender) Send(conn io.Writer, files map[string]*FileHeader) (*summary, error) {
	summ := &summary{
		files:       make([]*FileHeader, 0),
		failedFiles: make([]*FileHeader, 0),
	}

	for _, file := range files {
		_, err := s.WriteHeader(conn, file)
		if err != nil {
			summ.failedFiles = append(summ.failedFiles, file)
			summ.nFailedBytes += file.size
			return summ, err
		}

		f, err := file.Open()
		if err != nil {
			summ.failedFiles = append(summ.failedFiles, file)
			summ.nFailedBytes += file.size
			return summ, err
		}

		written, err := s.WriteFile(conn, f, file)
		f.Close()

		if err != nil {
			return summ, err
		}

		if written != file.size {
			return summ, fmt.Errorf("[err] corrupted: file %s expected %d bytes, wrote %d",
				file.name, file.size, written)
		}

		summ.files = append(summ.files, file)
		summ.nBytes += written
	}

	_, err := s.WriteEnd(conn)
	if err != nil {
		log.Println("[warn] failed to write end, but all files are written")
	}

	return summ, nil
}

func (s *Sender) WriteEnd(conn io.Writer) (int, error) {
	return conn.Write(EndHeaderBytes)
}

func (s *Sender) WriteHeader(conn io.Writer, f *FileHeader) (int64, error) {
	encoded, err := f.Encoded()
	if err != nil {
		return 0, err
	}

	encodedBytes, ok := encoded.(*EncodedFileHeader)
	if !ok {
		return 0, errors.New("failed to type assert to EncodedFileHeader")
	}

	rd := bytes.NewReader(*encodedBytes)

	return io.Copy(conn, rd)
}

func (s *Sender) WriteFile(conn io.Writer, file io.Reader, h *FileHeader) (int64, error) {
	n, err := io.CopyN(conn, file, h.size)
	return n, err
}
