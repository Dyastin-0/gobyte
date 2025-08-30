package core

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

type Sender struct{}

func NewSender() *Sender {
	return &Sender{}
}

func (s *Sender) Send(conn io.Writer, files map[string]*FileHeader, rq *RequestHeader) (*SummaryHeader, error) {
	summ := &SummaryHeader{
		files:       make([]*FileHeader, 0),
		failedFiles: make([]*FileHeader, 0),
	}

	counter := 1

	for _, file := range files {
		_, err := s.WriteHeader(conn, file)
		if err != nil {
			fmt.Printf("[err]: %v\n", err)
			summ.failedFiles = append(summ.failedFiles, file)
			summ.nFailedBytes += float64(file.size) / 1048576.0
			continue
		}

		f, err := file.Open()
		if err != nil {
			summ.failedFiles = append(summ.failedFiles, file)
			summ.nFailedBytes += float64(file.size) / 1048576.0
			return summ, err
		}

		written, err := s.WriteFile(conn, f, file, rq, counter)
		f.Close()

		if err != nil {
			return summ, err
		}

		if written != file.size {
			return summ, fmt.Errorf("[err] corrupted: file %s expected %d bytes, wrote %d",
				file.name, file.size, written)
		}

		summ.files = append(summ.files, file)
		summ.nBytes += float64(written) / 1048576.0
		counter++
	}

	return summ, nil
}

func (s *Sender) WriteSummary(conn io.Writer, summ *SummaryHeader) (int, error) {
	encoded, err := summ.Encoded()
	if err != nil {
		return 0, err
	}

	encodedBytes, ok := encoded.(*EncodedSummaryHeader)
	if !ok {
		return 0, errors.New("failed to type assert to EncodedSummaryHeader")
	}

	return conn.Write(*encodedBytes)
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

func (s *Sender) WriteFile(conn io.Writer, file io.Reader, h *FileHeader, rq *RequestHeader, count int) (int64, error) {
	text := fmt.Sprintf("[%d/%d] Sending %s", count, rq.n, h.name)
	bar := DefaultBar(h.size, text)

	n, err := io.CopyN(io.MultiWriter(conn, bar), file, h.size)
	return n, err
}
