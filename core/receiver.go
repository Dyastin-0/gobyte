package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
)

type Receiver struct {
	dir       string
	proto     *Proto
	OnRequest func(req *Request) bool
}

func NewReceiver(dir string) *Receiver {
	return &Receiver{
		dir:       dir,
		proto:     NewProto(),
		OnRequest: OnRequest,
	}
}

func (r *Receiver) receive(rdw io.ReadWriter) error {
	counter := 1

	req := &Request{}
	for {
		buf := make([]byte, HeaderSize)
		_, err := io.ReadFull(rdw, buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		hd, err := r.proto.DeserializeHeader(buf)
		if err != nil {
			return err
		}

		switch hd.Type {
		case TypeRequest:
			var err error
			req, err = r.ReadRequest(rdw)
			if err != nil {
				return err
			}

			ok := r.OnRequest(req)
			if !ok {
				err = r.WriteResponse(rdw, TypeDenied)
				return err
			}

			err = r.WriteResponse(rdw, TypeAck)
			if err != nil {
				return err
			}

			err = r.ReadFiles(rdw, req, &counter)
			if err != nil {
				return err
			}

		default:
			r.WriteResponse(rdw, TypeError)
			return nil
		}
	}
}

func (r *Receiver) WriteResponse(w io.Writer, msgType uint8) error {
	header := NewHeader(msgType, 0)
	serializedheader, err := r.proto.SerializeHeader(header)
	if err != nil {
		return err
	}

	_, err = w.Write(serializedheader)
	return err
}

func (r *Receiver) ReadFiles(rd io.Reader, req *Request, counter *int) error {
	for {
		buf := make([]byte, HeaderSize)
		_, err := io.ReadFull(rd, buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		hd, err := r.proto.DeserializeHeader(buf)
		if err != nil {
			return err
		}

		switch hd.Type {
		case TypeFileMetadata:
			metadata, err := r.ReadFileMetadata(rd)
			if err != nil {
				return err
			}

			_, err = r.Write(rd, metadata, req, *counter)
			if err != nil {
				return err
			}

			*counter++

		case TypeEnd:
			return nil
		}
	}
}

func (r *Receiver) ReadRequest(rd io.Reader) (*Request, error) {
	buf := make([]byte, RequestSize)

	_, err := io.ReadFull(rd, buf)
	if err != nil {
		return nil, err
	}

	header, err := r.proto.DeserializeRequest(buf)
	if err != nil {
		return nil, err
	}

	return header, nil
}

func (r *Receiver) ReadFileMetadata(rd io.Reader) (*FileMetadata, error) {
	fixedBuf := make([]byte, FileMetadataSize)
	_, err := io.ReadFull(rd, fixedBuf)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(fixedBuf)
	var size uint64
	var lengthName, lengthPath uint32

	if err = binary.Read(reader, binary.BigEndian, &size); err != nil {
		return nil, fmt.Errorf("failed to read size: %w", err)
	}
	if err = binary.Read(reader, binary.BigEndian, &lengthName); err != nil {
		return nil, fmt.Errorf("failed to read name length: %w", err)
	}
	if err = binary.Read(reader, binary.BigEndian, &lengthPath); err != nil {
		return nil, fmt.Errorf("failed to read path length: %w", err)
	}

	stringDataSize := int(lengthName) + int(lengthPath)
	stringBuf := make([]byte, stringDataSize)
	if stringDataSize > 0 {
		_, err = io.ReadFull(rd, stringBuf)
		if err != nil {
			return nil, err
		}
	}

	data := make([]byte, len(fixedBuf)+len(stringBuf))
	copy(data, fixedBuf)
	copy(data[len(fixedBuf):], stringBuf)

	// Deserialize the complete metadata
	metadata, err := r.proto.DeserializeFileMetadata(data)
	if err != nil {
		return nil, err
	}

	return metadata, nil
}

func (r *Receiver) Write(rd io.Reader, metadata *FileMetadata, req *Request, counter int) (int64, error) {
	dir := filepath.Join(r.dir, metadata.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, err
	}

	filePath := filepath.Join(r.dir, metadata.Path, metadata.Name)

	_, err := os.Stat(filePath)
	if err == nil {
		ext := filepath.Ext(metadata.Name)
		nameWithoutExt := (metadata.Name)[:len(metadata.Name)-len(ext)]
		var c int
		c, err = countSameFileNamePrefix(metadata.Path, nameWithoutExt, ext)
		if err != nil {
			return 0, err
		}

		filePath = filepath.Join(r.dir, metadata.Path, fmt.Sprintf("%s (%d)%s", nameWithoutExt, c+1, ext))
	}

	file, err := os.Create(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	text := fmt.Sprintf("[%d/%d] Writing %s", counter, req.Length, metadata.Name)
	bar := DefaultBar(int64(metadata.Size), text)

	n, err := io.CopyN(io.MultiWriter(file, bar), rd, int64(metadata.Size))
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

func OnRequest(req *Request) bool {
	confirm := false

	title := fmt.Sprintf("Accept %d files? (%d Bytes) \n", req.Length, req.Size)

	huh.NewConfirm().
		Title(title).
		Affirmative("Yes").
		Negative("No").
		Value(&confirm).
		Run()

	return confirm
}
