package core

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSendMany(t *testing.T) {
	dir := t.TempDir()
	dir1 := t.TempDir()
	s := NewSender()
	r := NewReceiver(dir1)
	r.OnRequest = func(req *Request) bool { return true }
	size, metadata, err := createNFiles(1000, dir)
	require.NoError(t, err)

	sender, receiver := net.Pipe()
	defer sender.Close()
	defer receiver.Close()

	go r.receive(sender)

	req := NewRequest(size, uint32(len(metadata)))
	s.WriteRequest(receiver, req)

	err = s.ReadResponse(receiver)
	require.NoError(t, err)

	err = s.Send(receiver, metadata, req)
	require.NoError(t, err)

	err = s.WriteEnd(receiver)
	require.NoError(t, err)
}

func createNFiles(n int, dir string) (uint64, map[string]*FileMetadata, error) {
	files := make(map[string]*FileMetadata, n)

	var sumBytes int64

	for i := range n {
		filename := fmt.Sprintf("testfile_%d.txt", i)
		filepath := filepath.Join(dir, filename)

		content := fmt.Appendf(nil, "test content %d", i)
		if err := os.WriteFile(filepath, content, 0644); err != nil {
			return 0, nil, err
		}

		fileStat, err := os.Stat(filepath)
		if err != nil {
			return 0, nil, err
		}

		sumBytes += fileStat.Size()

		files[filepath] = &FileMetadata{
			Size:       uint64(fileStat.Size()),
			LengthName: uint32(len(fileStat.Name())),
			LengthPath: uint32(len(dir)),
			Name:       fileStat.Name(),
			Path:       dir,
			AbsPath:    filepath,
		}
	}

	return uint64(sumBytes), files, nil
}
