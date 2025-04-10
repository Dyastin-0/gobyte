package gobyte

import (
	"github.com/charmbracelet/lipgloss"
)

const (
	discoveryPort = 8888
	transferPort  = 8889
	broadcastAddr = "255.255.255.255"
	discoveryMsg  = "GOBYTE"
	maxBufferSize = 1024 * 1024
)

type Peer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
}

type FileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Path string `json:"path"`
}

type MessageType string

const (
	TypeDiscovery    MessageType = "discovery"
	TypeDiscoveryAck MessageType = "discovery_ack"
	TypeTransferReq  MessageType = "transfer_req"
	TypeTransferAck  MessageType = "transfer_ack"
)

type Message struct {
	Type       MessageType `json:"type"`
	SenderID   string      `json:"sender_id"`
	SenderName string      `json:"sender_name"`
	IPAddress  string      `json:"ip_address"`
	Files      []FileInfo  `json:"files,omitempty"`
	Peers      []string    `json:"peers,omitempty"`
}

var (
	TITLE = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	INFO = lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#888888"))
	SUCCESS = lipgloss.NewStyle().Foreground(lipgloss.Color("#28a745"))
)
