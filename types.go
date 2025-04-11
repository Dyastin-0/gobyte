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
	TypeDiscovery    = "discovery"
	TypeDiscoveryAck = "discovery_ack"
	TypeTransferReq  = "transfer_req"
	TypeTransferAck  = "transfer_ack"
)

type Message struct {
	Type       string     `json:"type"`
	SenderID   string     `json:"sender_id"`
	SenderName string     `json:"sender_name"`
	IPAddress  string     `json:"ip_address"`
	Files      []FileInfo `json:"files,omitempty"`
	Accepted   bool       `json:"accepted,omitempty"`
	TransferID string     `json:"transfer_id,omitempty"`
}

var (
	TITLE = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7d56f4"))

	INFO = lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#888888"))

	SUCCESS = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#28a745"))

	ERROR = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ee4b2b"))
)
