package types

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
	TypeUDPreq      = "udp_req"
	TypeUDPping     = "udp_ping"
	TypeUDPpong     = "udp_pong"
	TypeTransferReq = "transfer_req"
	TypeTransferAck = "transfer_ack"
)

type Message struct {
	Type       string `json:"type"`
	SenderID   string `json:"sender_id"`
	SenderName string `json:"sender_name"`
	IPAddress  string `json:"ip_address"`
	Len        int    `json:"len,omitempty"`
	Accepted   bool   `json:"accepted,omitempty"`
	Reason     string `json:"reason,omitempty"`
	TransferID string `json:"transfer_id,omitempty"`
}
