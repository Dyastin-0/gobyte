package core

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Receiver struct {
	dir string
}

func NewReceiver(dir string) *Receiver {
	return &Receiver{
		dir: dir,
	}
}

func (r *Receiver) receive(rd io.Reader, rq *RequestHeader) error {
	reader := bufio.NewReader(rd)
	counter := 1

	for {
		header, err := reader.ReadString(delim)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		encodedSummary := EncodedSummaryHeader(header)
		summ, err := encodedSummary.Parse()
		if err == nil {
			parsedSumm, ok := summ.(*SummaryHeader)
			if !ok {
				continue
			}

			fmt.Printf(
				"[Summary] Received %f MB\n[Summary] Failed %f MB\n",
				parsedSumm.nBytes,
				parsedSumm.nFailedBytes,
			)
			continue
		}

		encodedEnd := EncodedEndHeader(header)
		_, err = encodedEnd.Parse()
		if err == nil {
			// return if header is EndHeader
			return nil
		}

		encoded := EncodedFileHeader(header)
		h, err := encoded.Parse()
		if err != nil {
			// if current header is malformed, read until the next header again
			continue
		}

		parsedHeader, ok := h.(*FileHeader)
		if !ok {
			continue
		}

		_, err = r.Write(reader, parsedHeader, rq, counter)
		if err != nil {
			return nil
		}

		counter++
	}
}

func (r *Receiver) Write(rd io.Reader, h *FileHeader, rq *RequestHeader, counter int) (int64, error) {
	path := filepath.Join(r.dir, h.path)
	if err := os.MkdirAll(path, 0755); err != nil {
		return 0, err
	}

	filePath := filepath.Join(path, h.name)

	_, err := os.Stat(filePath)
	if err == nil {
		ext := filepath.Ext(h.name)
		nameWithoutExt := (h.name)[:len(h.name)-len(ext)]
		var c int
		c, err = countSameFileNamePrefix(path, nameWithoutExt, ext)
		if err != nil {
			return 0, err
		}

		filePath = filepath.Join(path, fmt.Sprintf("%s (%d)%s", nameWithoutExt, c+1, ext))
	}

	file, err := os.Create(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	text := fmt.Sprintf("[%d/%d] Writing %s", counter, rq.n, h.name)
	bar := DefaultBar(h.size, text)

	n, err := io.CopyN(io.MultiWriter(file, bar), rd, h.size)
	return n, err
}

func countSameFileNamePrefix(dir, prefix, ext string) (int, error) {
	pattern := filepath.Join(dir, fmt.Sprintf("%s ([0-9]*)%s", prefix, ext))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return 0, err
	}
	return len(matches), nil
}
