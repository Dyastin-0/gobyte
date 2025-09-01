package core

import (
	"fmt"
	"io"
	"net"
	"os"
)

type Sender struct {
	proto *Proto
}

func NewSender() *Sender {
	return &Sender{
		NewProto(),
	}
}

func (s *Sender) Send(conn io.Writer, fileMetadata map[string]*FileMetadata, req *Request) error {
	counter := 1

	for _, metadata := range fileMetadata {
		err := s.WriteHeader(conn, metadata)
		if err != nil {
			fmt.Printf("[err]: %v\n", err)
			continue
		}

		written, err := s.WriteFile(conn, metadata, req, counter)
		if err != nil {
			return err
		}

		if uint64(written) != metadata.Size {
			return fmt.Errorf("[err] corrupted: file %s expected %d bytes, wrote %d",
				metadata.Name, metadata.Size, written)
		}

		counter++
	}

	return nil
}

func (s *Sender) WriteRequest(conn net.Conn, req *Request) error {
	serialized, err := s.proto.SerializeRequest(req)
	if err != nil {
		return err
	}

	hd := NewHeader(TypeRequest, uint64(RequestSize))
	serializedHeader, err := s.proto.SerializeHeader(hd)
	if err != nil {
		return err
	}

	_, err = conn.Write(serializedHeader)
	if err != nil {
		return err
	}

	_, err = conn.Write(serialized)
	if err != nil {
		return err
	}

	return nil
}

func (s *Sender) ReadResponse(conn net.Conn) error {
	buf := make([]byte, HeaderSize)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		return err
	}

	dh, err := s.proto.DeserializeHeader(buf)
	if err != nil {
		return err
	}

	if dh.Type != TypeAck && dh.Type != TypeDenied {
		return ErrInvalidResponse
	}

	if dh.Type != TypeAck {
		return ErrRequestDenied
	}

	return nil
}

func (s *Sender) WriteEnd(w io.Writer) error {
	header := NewHeader(TypeEnd, 0)
	serializedHeader, err := s.proto.SerializeHeader(header)
	if err != nil {
		return err
	}

	_, err = w.Write(serializedHeader)

	return err
}

func (s *Sender) WriteHeader(w io.Writer, metadata *FileMetadata) error {
	header := NewHeader(TypeFileMetadata, metadata.Size)
	serializedHeader, err := s.proto.SerializeHeader(header)
	if err != nil {
		return err
	}

	_, err = w.Write(serializedHeader)
	if err != nil {
		return err
	}

	serializedMetadata, err := s.proto.SerializeFileMetadata(metadata)
	if err != nil {
		return err
	}

	_, err = w.Write(serializedMetadata)
	if err != nil {
		return err
	}

	return nil
}

func (s *Sender) WriteFile(conn io.Writer, metadata *FileMetadata, req *Request, count int) (int64, error) {
	file, err := os.Open(metadata.AbsPath)
	if err != nil {
		return 0, err
	}

	text := fmt.Sprintf("[%d/%d] Sending %s", count, req.Length, metadata.Name)
	bar := DefaultBar(int64(metadata.Size), text)

	return io.CopyN(io.MultiWriter(conn, bar), file, int64(metadata.Size))
}
