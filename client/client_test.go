package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dyastin-0/gobyte/progress"
	"github.com/Dyastin-0/gobyte/types"
	"github.com/stretchr/testify/assert"
)

func TestReadFileHeader(t *testing.T) {
	fileName, fileSize, err := readFileHeader("FILE:example.txt:1024")
	assert.Nil(t, err)
	assert.Equal(t, "example.txt", fileName)
	assert.Equal(t, int64(1024), fileSize)

	_, _, err = readFileHeader("WRONGPREFIX:example.txt:1024")
	assert.Error(t, err)

	_, _, err = readFileHeader("FILE:example.txt")
	assert.Error(t, err)

	_, _, err = readFileHeader("FILE:example.txt:notanumber")
	assert.Error(t, err)
}

func TestWriteBytesToDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test")
	defer os.RemoveAll(tempDir)
	assert.Nil(t, err)

	testData := []byte("test content")
	reader := bytes.NewReader(testData)

	written, err := writeBytesToDir(reader, int64(len(testData)), tempDir, "test.txt")
	assert.Nil(t, err)
	assert.Equal(t, int64(len(testData)), written)

	content, err := os.ReadFile(filepath.Join(tempDir, "test.txt"))
	assert.Nil(t, err)
	assert.Equal(t, testData, content)

	reader = bytes.NewReader(testData)
	written, err = writeBytesToDir(reader, int64(len(testData)), tempDir, "test.txt")
	assert.Nil(t, err)
	assert.Equal(t, int64(len(testData)), written)

	files, err := filepath.Glob(filepath.Join(tempDir, "test_*.txt"))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(files))
}

func TestSendAck(t *testing.T) {
	mockAddr := "127.0.0.1:8000"
	addr, _ := net.ResolveUDPAddr("udp", mockAddr)
	listener, _ := net.ListenUDP("udp", addr)
	defer listener.Close()

	client := &Client{
		Self: &types.Peer{
			ID:        "test-id",
			Name:      "test-name",
			IPAddress: "127.0.0.1",
		},
		discoveryPort: 8000,
	}

	msg := types.Message{
		IPAddress:  "127.0.0.1",
		TransferID: "test-transfer",
	}

	go func() {
		buf := make([]byte, 1024)
		listener.SetReadDeadline(time.Now().Add(time.Second))
		n, _, _ := listener.ReadFromUDP(buf)

		if n > 0 {
			var receivedMsg types.Message
			json.Unmarshal(buf[:n], &receivedMsg)
			assert.Equal(t, types.TypeTransferAck, receivedMsg.Type)
			assert.Equal(t, "test-id", receivedMsg.SenderID)
			assert.Equal(t, "test-transfer", receivedMsg.TransferID)
			assert.True(t, receivedMsg.Accepted)
		}
	}()

	err := client.sendAck(msg, "", true)
	assert.Nil(t, err)
}

type mockConn struct {
	readData []byte
	readPos  int
	net.Conn
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	if m.readPos >= len(m.readData) {
		return 0, io.EOF
	}
	n = copy(b, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}

func (m *mockConn) Close() error {
	return nil
}

func TestReadFiles(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "test")
	defer os.RemoveAll(tempDir)

	fileContent := "test file content"
	headerStr := fmt.Sprintf("FILE:test.txt:%d\n", len(fileContent))
	mockData := []byte(headerStr + fileContent + "END\n")
	conn := &mockConn{readData: mockData}

	client := &Client{}

	err := client.readFiles(conn, tempDir)
	assert.Nil(t, err)

	content, err := os.ReadFile(filepath.Join(tempDir, "test.txt"))
	assert.Nil(t, err)
	assert.Equal(t, []byte(fileContent), content)

	file1Content := "file 1 content"
	file2Content := "file 2 content is longer"
	header1 := fmt.Sprintf("FILE:file1.txt:%d\n", len(file1Content))
	header2 := fmt.Sprintf("FILE:file2.txt:%d\n", len(file2Content))
	mockData = []byte(header1 + file1Content + header2 + file2Content + "END\n")
	conn = &mockConn{readData: mockData}

	err = client.readFiles(conn, tempDir)
	assert.Nil(t, err)

	content1, _ := os.ReadFile(filepath.Join(tempDir, "file1.txt"))
	content2, _ := os.ReadFile(filepath.Join(tempDir, "file2.txt"))
	assert.Equal(t, []byte(file1Content), content1)
	assert.Equal(t, []byte(file2Content), content2)
}

func TestWriteFileHeader(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)

	fileInfo := types.FileInfo{
		Name: "test.txt",
		Size: 1024,
	}

	err := writeFileHeader(writer, fileInfo)
	assert.Nil(t, err)
	writer.Flush()

	expected := "FILE:test.txt:1024\n"
	assert.Equal(t, expected, buf.String())
}

func TestCopyN(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test.txt")
	assert.Nil(t, err)
	defer os.Remove(tempFile.Name())

	testContent := "test file content"
	_, err = tempFile.WriteString(testContent)
	assert.Nil(t, err)
	tempFile.Close()

	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)

	peer := types.Peer{
		ID:        "test",
		Name:      "test peer",
		IPAddress: "127.0.0.1",
	}

	fileInfo := types.FileInfo{
		Name: "test.txt",
		Path: tempFile.Name(),
		Size: int64(len(testContent)),
	}

	p := progress.New()

	written, err := copyN(writer, fileInfo, peer, p)
	assert.Nil(t, err)
	writer.Flush()

	assert.Equal(t, int64(len(testContent)), written)

	expectedHeader := fmt.Sprintf("FILE:test.txt:%d\n", len(testContent))
	expectedContent := expectedHeader + testContent
	assert.Equal(t, expectedContent, buf.String())
}

func TestSendTransferReq(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	assert.Nil(t, err)

	server, err := net.ListenUDP("udp", addr)
	assert.Nil(t, err)
	defer server.Close()

	localAddr := server.LocalAddr().(*net.UDPAddr)

	peer := types.Peer{
		ID:        "test-peer",
		Name:      "Test Peer",
		IPAddress: "127.0.0.1",
	}

	client := &Client{
		Self: &types.Peer{
			ID:        "test-id",
			Name:      "Test Client",
			IPAddress: "127.0.0.1",
		},
		discoveryPort: localAddr.Port,
	}

	files := []types.FileInfo{
		{Name: "test.txt", Path: "/path/to/test.txt", Size: 1024},
	}

	receivedChan := make(chan types.Message)
	go func() {
		buf := make([]byte, 1024)
		server.SetReadDeadline(time.Now().Add(time.Second))
		n, _, err := server.ReadFromUDP(buf)
		if err != nil {
			t.Logf("Error reading UDP: %v", err)
			return
		}

		var msg types.Message
		if err := json.Unmarshal(buf[:n], &msg); err != nil {
			t.Logf("Error unmarshaling: %v", err)
			return
		}

		receivedChan <- msg
	}()

	transferID := "test-transfer-id"
	err = client.sendTransferReq(peer, files, transferID)
	assert.Nil(t, err)

	select {
	case msg := <-receivedChan:
		assert.Equal(t, types.TypeTransferReq, msg.Type)
		assert.Equal(t, client.Self.ID, msg.SenderID)
		assert.Equal(t, client.Self.Name, msg.SenderName)
		assert.Equal(t, transferID, msg.TransferID)
		assert.Equal(t, 1, len(msg.Files))
		assert.Equal(t, "test.txt", msg.Files[0].Name)
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for transfer request")
	}
}

func TestChuckFilesToPeers(t *testing.T) {
	client := &Client{}

	peersReceived := make(map[string]bool)

	client.writeFilesFunc = func(p types.Peer, f []types.FileInfo) error {
		peersReceived[p.ID] = true
		return nil
	}

	peers := []types.Peer{
		{ID: "peer1", Name: "Peer 1", IPAddress: "127.0.0.1"},
		{ID: "peer2", Name: "Peer 2", IPAddress: "127.0.0.2"},
	}

	files := []types.FileInfo{
		{Name: "test.txt", Path: "/path/to/test.txt", Size: 1024},
	}

	err := client.ChuckFilesToPeers(peers, files)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(peersReceived))
	assert.True(t, peersReceived["peer1"])
	assert.True(t, peersReceived["peer2"])
}

func TestListen(t *testing.T) {
	client := &Client{
		Self:       &types.Peer{ID: "self"},
		knownPeers: make(map[string]types.Peer),
	}

	msg := types.Message{
		Type:       types.TypeUDPreq,
		SenderID:   "other",
		SenderName: "Peer Name",
		IPAddress:  "192.168.1.10",
	}

	client.handleUDPMessage(msg)

	if _, ok := client.knownPeers["other"]; !ok {
		t.Errorf("expected peer to be added")
	}
}

func TestHandlePingResponse_RemovesPeerIfNoResponse(t *testing.T) {
	c := &Client{
		knownPeers:  make(map[string]types.Peer),
		pendingPong: make(map[string]chan bool),
	}

	c.knownPeers["peer1"] = types.Peer{
		ID:        "peer1",
		Name:      "Tester",
		IPAddress: "192.168.1.1",
	}

	c.handlePingResponse("peer1", false)

	if _, exists := c.knownPeers["peer1"]; exists {
		t.Errorf("peer1 should have been removed after no pong")
	}

	c.knownPeers["peer1"] = types.Peer{
		ID:        "peer1",
		Name:      "Tester",
		IPAddress: "192.168.1.1",
	}

	c.handlePingResponse("peer1", true)

	if _, exists := c.knownPeers["peer1"]; !exists {
		t.Errorf("peer1 should NOT have been removed")
	}
}

func TestBroadcastSelf(t *testing.T) {
	buf := &bytes.Buffer{}

	c := &Client{
		Self: &types.Peer{
			ID:        "xyz",
			Name:      "test-user",
			IPAddress: "10.0.0.1",
		},
	}

	c.broadcastSelf(buf)

	var msg types.Message
	if err := json.Unmarshal(buf.Bytes(), &msg); err != nil {
		t.Fatalf("failed to unmarshal written data: %v", err)
	}

	if msg.Type != types.TypeUDPreq || msg.SenderID != "xyz" || msg.SenderName != "test-user" {
		t.Errorf("unexpected broadcast message: %+v", msg)
	}
}
