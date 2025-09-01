package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	TypeRequest      uint8 = 0x01
	TypeFileMetadata uint8 = 0x02
	TypeAck          uint8 = 0x03
	TypeEnd          uint8 = 0x04
	TypeHello        uint8 = 0x05
	TypeDenied       uint8 = 0x06
	TypeError        uint8 = 0xFF

	MaxPayloadSize   uint64 = 32 * 1024 * 1024 * 1024 // 32 GB
	MaxStringLength  uint32 = 4096                    // 4 KB max for paths/names
	MaxFileNumber    uint32 = 1000000
	HeaderSize       uint8  = 12
	RequestSize      uint8  = 12
	FileMetadataSize uint8  = 16

	Version uint8 = 0x11
	VERSION       = "1.1"
)

var (
	ErrInvalidVersion      = errors.New("invalid version")
	ErrInvalidType         = errors.New("invalid type")
	ErrReservedFieldUsed   = errors.New("reserved field must be zero")
	ErrPayloadTooLarge     = errors.New("payload exceeds maximum size")
	ErrStringTooLong       = errors.New("string exceeds maximum length")
	ErrInvalidHeaderSize   = errors.New("header data too small")
	ErrInvalidRequestSize  = errors.New("request data too small")
	ErrInvalidMetadataSize = errors.New("file metadata data too small")
	ErrInvalidLength       = errors.New("invalid length field")
	ErrInsufficientData    = errors.New("insufficient data for string fields")
	ErrEmptyString         = errors.New("string field cannot be empty")
)

// Header represents the protocol header (12 bytes)
type Header struct {
	Version  uint8  // 1 byte
	Type     uint8  // 1 byte
	Length   uint64 // 8 bytes
	Reserved uint16 // 2 bytes
}

// Request represents a request message payload (12 bytes)
type Request struct {
	Size   uint64 // 8 bytes
	Length uint32 // 4 bytes
}

// FileMetadata represents file metadata payload (16 bytes + variable strings)
type FileMetadata struct {
	Size       uint64 // 8 bytes
	LengthName uint32 // 4 bytes
	LengthPath uint32 // 4 bytes
	Name       string // filename (max 4KB)
	Path       string // file path (max 4KB)
	AbsPath    string // not serialized - absolute path
}

// Proto handles protocol serialization and deserialization
type Proto struct{}

// NewProto creates a new protocol handler
func NewProto() *Proto {
	return &Proto{}
}

// SerializeHeader serializes a header to bytes
func (p *Proto) SerializeHeader(header *Header) ([]byte, error) {
	if err := p.validateHeader(header); err != nil {
		return nil, fmt.Errorf("header validation failed: %w", err)
	}

	buf := bytes.NewBuffer(make([]byte, 0, HeaderSize))

	if err := binary.Write(buf, binary.BigEndian, header.Version); err != nil {
		return nil, fmt.Errorf("failed to write version: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, header.Type); err != nil {
		return nil, fmt.Errorf("failed to write type: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, header.Length); err != nil {
		return nil, fmt.Errorf("failed to write length: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, header.Reserved); err != nil {
		return nil, fmt.Errorf("failed to write reserved: %w", err)
	}

	return buf.Bytes(), nil
}

// DeserializeHeader deserializes bytes to a header
func (p *Proto) DeserializeHeader(data []byte) (*Header, error) {
	if len(data) < int(HeaderSize) {
		return nil, ErrInvalidHeaderSize
	}

	reader := bytes.NewReader(data[:HeaderSize])
	var header Header

	if err := binary.Read(reader, binary.BigEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	if err := p.validateHeader(&header); err != nil {
		return nil, fmt.Errorf("header validation failed: %w", err)
	}

	return &header, nil
}

// SerializeRequest serializes a request to bytes
func (p *Proto) SerializeRequest(req *Request) ([]byte, error) {
	if err := p.validateRequest(req); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	buf := bytes.NewBuffer(make([]byte, 0, RequestSize))

	if err := binary.Write(buf, binary.BigEndian, req.Size); err != nil {
		return nil, fmt.Errorf("failed to write size: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, req.Length); err != nil {
		return nil, fmt.Errorf("failed to write length: %w", err)
	}

	return buf.Bytes(), nil
}

// DeserializeRequest deserializes bytes to a request
func (p *Proto) DeserializeRequest(data []byte) (*Request, error) {
	if len(data) < int(RequestSize) {
		return nil, ErrInvalidRequestSize
	}

	reader := bytes.NewReader(data[:RequestSize])
	var req Request

	if err := binary.Read(reader, binary.BigEndian, &req); err != nil {
		return nil, fmt.Errorf("failed to read request: %w", err)
	}

	if err := p.validateRequest(&req); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	return &req, nil
}

// SerializeFileMetadata serializes file metadata to bytes
func (p *Proto) SerializeFileMetadata(fm *FileMetadata) ([]byte, error) {
	if err := p.validateFileMetadata(fm); err != nil {
		return nil, fmt.Errorf("file metadata validation failed: %w", err)
	}

	totalSize := int(FileMetadataSize) + len(fm.Name) + len(fm.Path)
	buf := bytes.NewBuffer(make([]byte, 0, totalSize))

	if err := binary.Write(buf, binary.BigEndian, fm.Size); err != nil {
		return nil, fmt.Errorf("failed to write size: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, fm.LengthName); err != nil {
		return nil, fmt.Errorf("failed to write name length: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, fm.LengthPath); err != nil {
		return nil, fmt.Errorf("failed to write path length: %w", err)
	}

	if _, err := buf.WriteString(fm.Name); err != nil {
		return nil, fmt.Errorf("failed to write name: %w", err)
	}
	if _, err := buf.WriteString(fm.Path); err != nil {
		return nil, fmt.Errorf("failed to write path: %w", err)
	}

	return buf.Bytes(), nil
}

// DeserializeFileMetadata deserializes bytes to file metadata
func (p *Proto) DeserializeFileMetadata(data []byte) (*FileMetadata, error) {
	if len(data) < int(FileMetadataSize) {
		return nil, ErrInvalidMetadataSize
	}

	reader := bytes.NewReader(data)
	var fm FileMetadata

	if err := binary.Read(reader, binary.BigEndian, &fm.Size); err != nil {
		return nil, fmt.Errorf("failed to read size: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &fm.LengthName); err != nil {
		return nil, fmt.Errorf("failed to read name length: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &fm.LengthPath); err != nil {
		return nil, fmt.Errorf("failed to read path length: %w", err)
	}

	expectedSize := int(FileMetadataSize) + int(fm.LengthName) + int(fm.LengthPath)
	if len(data) < expectedSize {
		return nil, ErrInsufficientData
	}

	nameBytes := make([]byte, fm.LengthName)
	if n, err := reader.Read(nameBytes); err != nil || n != int(fm.LengthName) {
		return nil, fmt.Errorf("failed to read name: %w", err)
	}
	fm.Name = string(nameBytes)

	pathBytes := make([]byte, fm.LengthPath)
	if n, err := reader.Read(pathBytes); err != nil || n != int(fm.LengthPath) {
		return nil, fmt.Errorf("failed to read path: %w", err)
	}
	fm.Path = string(pathBytes)

	if err := p.validateFileMetadata(&fm); err != nil {
		return nil, fmt.Errorf("file metadata validation failed: %w", err)
	}

	return &fm, nil
}

func (p *Proto) validateHeader(header *Header) error {
	if header.Version != Version {
		return ErrInvalidVersion
	}

	if header.Length > MaxPayloadSize {
		return ErrPayloadTooLarge
	}

	switch header.Type {
	case TypeRequest, TypeFileMetadata, TypeAck, TypeEnd, TypeError:
		// Valid types
	default:
		return ErrInvalidType
	}

	if header.Reserved != 0 {
		return ErrReservedFieldUsed
	}

	return nil
}

func (p *Proto) validateRequest(req *Request) error {
	if req.Length == 0 {
		return ErrInvalidLength
	}

	if req.Length > MaxFileNumber {
		return ErrInvalidLength
	}

	return nil
}

func (p *Proto) validateFileMetadata(fm *FileMetadata) error {
	if fm.LengthName == 0 || len(fm.Name) == 0 {
		return ErrEmptyString
	}
	if fm.LengthPath == 0 || len(fm.Path) == 0 {
		return ErrEmptyString
	}

	if fm.LengthName != uint32(len(fm.Name)) {
		return ErrInvalidLength
	}
	if fm.LengthPath != uint32(len(fm.Path)) {
		return ErrInvalidLength
	}

	if fm.LengthName > MaxStringLength {
		return ErrStringTooLong
	}
	if fm.LengthPath > MaxStringLength {
		return ErrStringTooLong
	}

	return nil
}

func (p *Proto) IsValidType(msgType uint8) bool {
	switch msgType {
	case TypeRequest, TypeFileMetadata, TypeAck, TypeEnd, TypeError:
		return true
	default:
		return false
	}
}

func NewHeader(msgType uint8, length uint64) *Header {
	return &Header{
		Version:  Version,
		Type:     msgType,
		Length:   length,
		Reserved: 0,
	}
}

func NewRequest(size uint64, length uint32) *Request {
	return &Request{
		Size:   size,
		Length: length,
	}
}

func NewFileMetadata(size uint64, name, path string) *FileMetadata {
	return &FileMetadata{
		Size:       size,
		LengthName: uint32(len(name)),
		LengthPath: uint32(len(path)),
		Name:       name,
		Path:       path,
	}
}
