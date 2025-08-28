package gobyte

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/vbauerster/mpb/v8"
)

type Receiver struct {
	dir string
	p   *mpb.Progress
}

func NewReceiver(dir string) *Receiver {
	return &Receiver{
		dir: dir,
		p:   mpb.New(),
	}
}

func (r *Receiver) Listen(ctx context.Context, ln net.Listener) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn, err := ln.Accept()
			if err != nil && err == io.EOF {
				return err
			}

			go func(conn net.Conn) {
				defer conn.Close()

				err := r.receive(conn)
				if err != nil {
					log.Printf("[err] %v\n", err)
				}
			}(conn)
		}
	}
}

func (r *Receiver) receive(rd io.Reader) error {
	reader := bufio.NewReader(rd)

	for {
		header, err := reader.ReadString(delim)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		encodedEnd := EncodedEndHeader(header)
		_, err = encodedEnd.Parse()
		if err == nil {
			// return if header is EndHeader
			return nil
		}

		encoded := EncodedHeader(header)
		h, err := encoded.Parse()
		if err != nil {
			// if current header is malformed, read until the next header again
			continue
		}

		parsedHeader, ok := h.(*FileHeader)
		if !ok {
			continue
		}

		_, err = r.Write(reader, parsedHeader)
		if err != nil {
			return nil
		}
	}
}

func (r *Receiver) Write(rd io.Reader, h *FileHeader) (int64, error) {
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

	bar := DefaultBar(h.size, h.name, r.p)
	proxy := bar.ProxyReader(rd)

	n, err := io.CopyN(file, proxy, h.size)

	bar.Wait()

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
