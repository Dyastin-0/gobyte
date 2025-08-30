package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSend(t *testing.T) {
	dir := t.TempDir()
	s := NewSender()

	testFilePath := filepath.Join(dir, "test.txt")
	err := os.WriteFile(testFilePath, []byte("test content"), 0644)
	if err != nil {
		t.Error(err)
	}

	fileStat, err := os.Stat(testFilePath)
	if err != nil {
		t.Error(err)
	}

	fileHeader := &FileHeader{
		name:    fileStat.Name(),
		size:    fileStat.Size(),
		path:    dir,
		abspath: testFilePath,
	}

	receiver, sender := io.Pipe()
	go func() {
		io.Copy(io.Discard, receiver)
	}()

	rq := &RequestHeader{n: 1}

	summ, err := s.Send(sender, map[string]*FileHeader{"test.txt": fileHeader}, rq)
	if err != nil {
		t.Error(err)
	}
	receiver.Close()

	assert.Equal(t, float64(fileHeader.size)/1048576.0, summ.nBytes)
}

func TestSendReceive(t *testing.T) {
	dir := t.TempDir()
	dir1 := t.TempDir()
	s := NewSender()
	r := NewReceiver(dir1)

	testFilePath := filepath.Join(dir, "test.txt")
	err := os.WriteFile(testFilePath, []byte("test content"), 0644)
	if err != nil {
		t.Error(err)
	}

	fileStat, err := os.Stat(testFilePath)
	if err != nil {
		t.Error(err)
	}

	fileHeader := &FileHeader{
		name:    fileStat.Name(),
		size:    fileStat.Size(),
		path:    dir,
		abspath: testFilePath,
	}

	receiver, sender := io.Pipe()
	defer receiver.Close()
	defer sender.Close()

	rq := &RequestHeader{n: 1}

	go r.receive(receiver, rq)

	summ, err := s.Send(sender, map[string]*FileHeader{"test.txt": fileHeader}, rq)
	if err != nil {
		t.Error(err)
	}
	receiver.Close()

	assert.Equal(t, float64(fileHeader.size)/1048576.0, summ.nBytes)

	time.Sleep(time.Millisecond * 100)

	receivedFilePath := filepath.Join(dir1, dir, "test.txt")
	receivedFileStat, err := os.Stat(receivedFilePath)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, summ.nBytes, float64(receivedFileStat.Size())/1048576.0)
}

func createNFiles(n int, dir string) (float64, map[string]*FileHeader, error) {
	files := make(map[string]*FileHeader, n)

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

		files[filepath] = &FileHeader{
			name:    fileStat.Name(),
			size:    fileStat.Size(),
			path:    dir,
			abspath: filepath,
		}
	}

	return float64(sumBytes) / 1048576.0, files, nil
}

func TestSendReceivedMany(t *testing.T) {
	dir := t.TempDir()
	dir1 := t.TempDir()
	s := NewSender()
	r := NewReceiver(dir1)

	sumBytes, files, err := createNFiles(1000, dir)
	if err != nil {
		t.Fatal(err)
	}

	receiver, sender := io.Pipe()
	defer receiver.Close()
	defer sender.Close()

	rq := &RequestHeader{n: 1}

	go r.receive(receiver, rq)

	summ, err := s.Send(sender, files, rq)
	receiver.Close()
	assert.NoError(t, err)

	assert.Equal(t, sumBytes, summ.nBytes)

	time.Sleep(time.Millisecond * 100)

	var receivedSumBytes int64
	for _, f := range files {
		receivedPath := filepath.Join(dir1, dir, f.name)
		stat, err := os.Stat(receivedPath)
		if err != nil {
			t.Errorf("expected file %s to exist, but got error: %v", receivedPath, err)
			continue
		}
		receivedSumBytes += stat.Size()
		assert.Equal(t, f.size, stat.Size())
	}

	assert.Equal(t, sumBytes, float64(receivedSumBytes)/1048576.0)
}
