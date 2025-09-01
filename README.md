# gobyte

`gobyte` is a command-line tool that enables fast and secure file transfers between devices on the same local network. It uses a custom binary file transfer protocol over TLS-encrypted TCP connections with a trust-on-first-use (TOFU) security mechanism.

## Demo

![demo](demo.gif)

## How It Works

### Connection

`gobyte` uses **trust-on-first-use** over TLS/TCP, similar to how SSH works. When establishing a connection, both peers must trust each other to proceed.

### Protocol

The current protocol version is `0x11` (`1.1`).
Every message begins with a fixed **12-byte header**.

```go
// Header (12 bytes)
type Header struct {
    Version  uint8  // must equal 0x11
    Type     uint8  // message type
    Length   uint64 // payload length in bytes
    Reserved uint16 // must be zero
}
```

#### Message Types

```go
const (
    TypeRequest      uint8 = 0x01 // announce transfer
    TypeFileMetadata uint8 = 0x02 // file metadata before file bytes
    TypeAck          uint8 = 0x03 // acknowledgment
    TypeEnd          uint8 = 0x04 // no more files
    TypeHello        uint8 = 0x05 // unused---maybe will be used for manually trusting a peer
    TypeDenied       uint8 = 0x06 // transfer denied
    TypeError        uint8 = 0xFF // error message
)
```

#### Payloads

**Request** – announces the upcoming transfer (fixed 12 bytes):

```go
type Request struct {
    Size   uint64 // total bytes to transfer
    Length uint32 // number of files
}
```

**FileMetadata** – describes a file (16 bytes + variable strings):

```go
type FileMetadata struct {
    Size       uint64 // file size in bytes
    LengthName uint32 // filename length
    LengthPath uint32 // relative path length
    Name       string // UTF-8 filename
    Path       string // UTF-8 relative path
    AbsPath    string // (not serialized) absolute path
}
```

After a `FileMetadata` message, the **raw file bytes** follow directly.

#### Example Flow

1. Sender → Receiver
   `Header{Type: TypeRequest}` + `Request{Size, Length}`

2. Receiver → Sender
   `Header{Type: TypeAck}`

3. For each file:

   * Sender → Receiver: `Header{Type: TypeFileMetadata}` + `FileMetadata`
   * Sender → Receiver: file bytes (exactly `FileMetadata.Size` long)
   * Receiver → Sender: `Header{Type: TypeAck}`

4. When finished:

   * Sender → Receiver: `Header{Type: TypeEnd}`
   * Receiver closes the connection

---

## Install

```bash
go install github.com/Dyastin-0/gobyte@latest
```

## Usage

Start as a receiver:

```bash
gobyte receive
```

Start as a sender:

```bash
gobyte send
```

---

Do you want me to also add a **Go snippet showing how to build a simple sender loop** (e.g. sending Request → FileMetadata → file bytes → End), so the README doubles as a reference implementation?
