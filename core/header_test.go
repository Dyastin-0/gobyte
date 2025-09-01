package core

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeaderValid(t *testing.T) {
	p := NewProto()
	tests := []struct {
		name    string
		header  *Header
		wantErr bool
	}{
		{
			name:    "type ack",
			header:  &Header{Version: Version, Type: TypeAck, Length: 0, Reserved: 0},
			wantErr: false,
		},
		{
			name:    "type err",
			header:  &Header{Version: Version, Type: TypeError, Length: 0, Reserved: 0},
			wantErr: false,
		},
		{
			name:    "type end",
			header:  &Header{Version: Version, Type: TypeEnd, Length: 0, Reserved: 0},
			wantErr: false,
		},
		{
			name:    "type request",
			header:  &Header{Version: Version, Type: TypeRequest, Length: MaxPayloadSize, Reserved: 0},
			wantErr: false,
		},
		{
			name:    "type file metadata",
			header:  &Header{Version: Version, Type: TypeFileMetadata, Length: MaxPayloadSize, Reserved: 0},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.SerializeHeader(tt.header)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHeaderInvalid(t *testing.T) {
	p := NewProto()
	tests := []struct {
		name    string
		header  *Header
		wantErr bool
	}{
		{
			name:    "invalid version",
			header:  &Header{Version: 0x99, Type: TypeAck, Length: 0, Reserved: 0},
			wantErr: true,
		},
		{
			name:    "invalid type",
			header:  &Header{Version: Version, Type: 0x99, Length: 0, Reserved: 0},
			wantErr: true,
		},
		{
			name:    "reserved field used",
			header:  &Header{Version: Version, Type: TypeAck, Length: 0, Reserved: 1},
			wantErr: true,
		},
		{
			name:    "payload too large",
			header:  &Header{Version: Version, Type: TypeAck, Length: MaxPayloadSize + 1, Reserved: 0},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.SerializeHeader(tt.header)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHeaderSerializeDeserialize(t *testing.T) {
	p := NewProto()

	tests := []struct {
		name   string
		header *Header
	}{
		{
			name:   "ack message",
			header: NewHeader(TypeAck, 0),
		},
		{
			name:   "request with payload",
			header: NewHeader(TypeRequest, 1024),
		},
		{
			name:   "file metadata",
			header: NewHeader(TypeFileMetadata, 500),
		},
		{
			name:   "end message",
			header: NewHeader(TypeEnd, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			serialized, err := p.SerializeHeader(tt.header)
			require.NoError(t, err)
			assert.Equal(t, HeaderSize, uint8(len(serialized)))

			// Deserialize
			deserialized, err := p.DeserializeHeader(serialized)
			require.NoError(t, err)
			assert.Equal(t, tt.header, deserialized)
		})
	}
}

func TestHeaderDeserializeInvalidData(t *testing.T) {
	p := NewProto()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "insufficient data",
			data: []byte{0x11, 0x01, 0x00},
		},
		{
			name: "partial header",
			data: []byte{0x11, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.DeserializeHeader(tt.data)
			assert.Error(t, err)
		})
	}
}

func TestRequestValid(t *testing.T) {
	p := NewProto()
	tests := []struct {
		name    string
		request *Request
		wantErr bool
	}{
		{
			name:    "single file",
			request: &Request{Size: 1024, Length: 1},
			wantErr: false,
		},
		{
			name:    "multiple files",
			request: &Request{Size: 1024 * 1024, Length: 100},
			wantErr: false,
		},
		{
			name:    "maximum files",
			request: &Request{Size: MaxPayloadSize, Length: MaxFileNumber},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.SerializeRequest(tt.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRequestInvalid(t *testing.T) {
	p := NewProto()
	tests := []struct {
		name    string
		request *Request
		wantErr bool
	}{
		{
			name:    "zero length",
			request: &Request{Size: 1024, Length: 0},
			wantErr: true,
		},
		{
			name:    "too many files",
			request: &Request{Size: 1024, Length: MaxFileNumber + 1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.SerializeRequest(tt.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRequestSerializeDeserialize(t *testing.T) {
	p := NewProto()

	tests := []struct {
		name    string
		request *Request
	}{
		{
			name:    "small request",
			request: NewRequest(420, 69),
		},
		{
			name:    "large request",
			request: NewRequest(1024*1024*1024, 50000),
		},
		{
			name:    "single file request",
			request: NewRequest(2048, 1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			serialized, err := p.SerializeRequest(tt.request)
			require.NoError(t, err)
			assert.Equal(t, RequestSize, uint8(len(serialized)))

			// Deserialize
			deserialized, err := p.DeserializeRequest(serialized)
			require.NoError(t, err)
			assert.Equal(t, tt.request, deserialized)
		})
	}
}

func TestRequestDeserializeInvalidData(t *testing.T) {
	p := NewProto()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "insufficient data",
			data: []byte{0x00, 0x00, 0x00, 0x00},
		},
		{
			name: "partial request",
			data: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x45},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.DeserializeRequest(tt.data)
			assert.Error(t, err)
		})
	}
}

func TestFileMetadataValid(t *testing.T) {
	p := NewProto()
	tests := []struct {
		name     string
		metadata *FileMetadata
		wantErr  bool
	}{
		{
			name:     "simple file",
			metadata: &FileMetadata{Size: 1024, LengthName: 8, LengthPath: 9, Name: "test.txt", Path: "/tmp/test"},
			wantErr:  false,
		},
		{
			name:     "long filename",
			metadata: NewFileMetadata(2048, "very_long_filename_that_is_still_valid.txt", "/home/user/documents/"),
			wantErr:  false,
		},
		{
			name:     "nested path",
			metadata: NewFileMetadata(0, "config.json", "/etc/app/config/"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.SerializeFileMetadata(tt.metadata)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFileMetadataInvalid(t *testing.T) {
	p := NewProto()
	longString := make([]byte, MaxStringLength+1)
	for i := range longString {
		longString[i] = 'a'
	}

	tests := []struct {
		name     string
		metadata *FileMetadata
		wantErr  bool
	}{
		{
			name:     "empty name",
			metadata: &FileMetadata{Size: 1024, LengthName: 0, LengthPath: 4, Name: "", Path: "/tmp"},
			wantErr:  true,
		},
		{
			name:     "empty path",
			metadata: &FileMetadata{Size: 1024, LengthName: 8, LengthPath: 0, Name: "test.txt", Path: ""},
			wantErr:  true,
		},
		{
			name:     "name too long",
			metadata: NewFileMetadata(1024, string(longString), "/tmp/"),
			wantErr:  true,
		},
		{
			name:     "path too long",
			metadata: NewFileMetadata(1024, "test.txt", string(longString)),
			wantErr:  true,
		},
		{
			name:     "length mismatch name",
			metadata: &FileMetadata{Size: 1024, LengthName: 5, LengthPath: 4, Name: "test.txt", Path: "/tmp"},
			wantErr:  true,
		},
		{
			name:     "length mismatch path",
			metadata: &FileMetadata{Size: 1024, LengthName: 8, LengthPath: 10, Name: "test.txt", Path: "/tmp"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.SerializeFileMetadata(tt.metadata)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFileMetadataSerializeDeserialize(t *testing.T) {
	p := NewProto()

	tests := []struct {
		name     string
		metadata *FileMetadata
	}{
		{
			name:     "simple file",
			metadata: NewFileMetadata(1024, "test.txt", "/home/user/"),
		},
		{
			name:     "binary file",
			metadata: NewFileMetadata(2048000, "image.png", "/var/www/images/"),
		},
		{
			name:     "config file",
			metadata: NewFileMetadata(512, "config.yaml", "/etc/myapp/"),
		},
		{
			name:     "unicode filename",
			metadata: NewFileMetadata(0, "测试文件.txt", "/tmp/unicode/"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			serialized, err := p.SerializeFileMetadata(tt.metadata)
			require.NoError(t, err)
			expectedSize := int(FileMetadataSize) + len(tt.metadata.Name) + len(tt.metadata.Path)
			assert.Equal(t, expectedSize, len(serialized))

			// Deserialize
			deserialized, err := p.DeserializeFileMetadata(serialized)
			require.NoError(t, err)

			// Compare all fields except AbsPath (not serialized)
			assert.Equal(t, tt.metadata.Size, deserialized.Size)
			assert.Equal(t, tt.metadata.Name, deserialized.Name)
			assert.Equal(t, tt.metadata.Path, deserialized.Path)
			assert.Equal(t, tt.metadata.LengthName, deserialized.LengthName)
			assert.Equal(t, tt.metadata.LengthPath, deserialized.LengthPath)
		})
	}
}

func TestFileMetadataDeserializeInvalidData(t *testing.T) {
	p := NewProto()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "insufficient header data",
			data: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			name: "missing string data",
			data: []byte{
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, // size
				0x00, 0x00, 0x00, 0x05, // name length = 5
				0x00, 0x00, 0x00, 0x04, // path length = 4
				// missing actual string data
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.DeserializeFileMetadata(tt.data)
			assert.Error(t, err)
		})
	}
}

func TestHeaderRequestPayload(t *testing.T) {
	p := NewProto()

	// Create request payload
	req := NewRequest(420, 69)
	serializedReq, err := p.SerializeRequest(req)
	require.NoError(t, err)

	// Create header for the request
	header := NewHeader(TypeRequest, uint64(len(serializedReq)))
	serializedHeader, err := p.SerializeHeader(header)
	require.NoError(t, err)

	// Verify sizes
	assert.Equal(t, HeaderSize+RequestSize, uint8(len(serializedHeader))+uint8(len(serializedReq)))

	// Test with pipe communication
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()

	// Write header
	go func() {
		defer w.Close()
		w.Write(serializedHeader)
		w.Write(serializedReq)
	}()

	// Read and verify header
	headerBuf := make([]byte, HeaderSize)
	n, err := io.ReadFull(r, headerBuf)
	require.NoError(t, err)
	require.Equal(t, int(HeaderSize), n)

	deserializedHeader, err := p.DeserializeHeader(headerBuf)
	require.NoError(t, err)
	assert.Equal(t, uint64(len(serializedReq)), deserializedHeader.Length)
	assert.Equal(t, TypeRequest, deserializedHeader.Type)

	// Read and verify request
	reqBuf := make([]byte, RequestSize)
	n, err = io.ReadFull(r, reqBuf)
	require.NoError(t, err)
	require.Equal(t, int(RequestSize), n)

	deserializedReq, err := p.DeserializeRequest(reqBuf)
	require.NoError(t, err)
	assert.Equal(t, req, deserializedReq)
}

func TestFullProtocolFlow(t *testing.T) {
	p := NewProto()

	// Test complete flow: Request -> FileMetadata -> Ack
	tests := []struct {
		name     string
		messages []interface{}
	}{
		{
			name: "single file transfer",
			messages: []interface{}{
				NewRequest(1024, 1),
				NewFileMetadata(1024, "test.txt", "/tmp/"),
				NewHeader(TypeAck, 0),
			},
		},
		{
			name: "multiple files transfer",
			messages: []interface{}{
				NewRequest(4096, 2),
				NewFileMetadata(2048, "file1.txt", "/home/user/"),
				NewFileMetadata(2048, "file2.txt", "/home/user/"),
				NewHeader(TypeEnd, 0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i, msg := range tt.messages {
				switch v := msg.(type) {
				case *Request:
					serialized, err := p.SerializeRequest(v)
					require.NoError(t, err, "Failed to serialize request %d", i)

					deserialized, err := p.DeserializeRequest(serialized)
					require.NoError(t, err, "Failed to deserialize request %d", i)
					assert.Equal(t, v, deserialized)

				case *FileMetadata:
					serialized, err := p.SerializeFileMetadata(v)
					require.NoError(t, err, "Failed to serialize file metadata %d", i)

					deserialized, err := p.DeserializeFileMetadata(serialized)
					require.NoError(t, err, "Failed to deserialize file metadata %d", i)
					assert.Equal(t, v.Size, deserialized.Size)
					assert.Equal(t, v.Name, deserialized.Name)
					assert.Equal(t, v.Path, deserialized.Path)

				case *Header:
					serialized, err := p.SerializeHeader(v)
					require.NoError(t, err, "Failed to serialize header %d", i)

					deserialized, err := p.DeserializeHeader(serialized)
					require.NoError(t, err, "Failed to deserialize header %d", i)
					assert.Equal(t, v, deserialized)
				}
			}
		})
	}
}

func TestUtilityMethods(t *testing.T) {
	p := NewProto()

	t.Run("IsValidType", func(t *testing.T) {
		validTypes := []uint8{TypeRequest, TypeFileMetadata, TypeAck, TypeEnd, TypeError}
		for _, typ := range validTypes {
			assert.True(t, p.IsValidType(typ), "Type %d should be valid", typ)
		}

		invalidTypes := []uint8{0x00, 0x99, 0xAA}
		for _, typ := range invalidTypes {
			assert.False(t, p.IsValidType(typ), "Type %d should be invalid", typ)
		}
	})

	t.Run("Constructor methods", func(t *testing.T) {
		header := NewHeader(TypeAck, 100)
		assert.Equal(t, Version, header.Version)
		assert.Equal(t, TypeAck, header.Type)
		assert.Equal(t, uint64(100), header.Length)
		assert.Equal(t, uint16(0), header.Reserved)

		request := NewRequest(2048, 5)
		assert.Equal(t, uint64(2048), request.Size)
		assert.Equal(t, uint32(5), request.Length)

		metadata := NewFileMetadata(1024, "test.txt", "/tmp/")
		assert.Equal(t, uint64(1024), metadata.Size)
		assert.Equal(t, "test.txt", metadata.Name)
		assert.Equal(t, "/tmp/", metadata.Path)
		assert.Equal(t, uint32(len("test.txt")), metadata.LengthName)
		assert.Equal(t, uint32(len("/tmp/")), metadata.LengthPath)
	})
}
