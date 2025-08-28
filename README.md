# gobyte

`gobyte` is a command-line tool that enables fast and secure file transfers between devices on the same local network. It uses a custom file transfer protocol over TLS-encrypted TCP connections with a trust-on-first-use (TOFU) security mechanism.

## How It Works

### Connection

`gobyte` uses `trust-on-first-use` over TLS/TCP similar to how SSH works, when establishing a connection both peer must trust each other to proceed.

### Protocol

Each connection must start with the sender sending a `RequestHeader` which is encoded as `0x1F<number of files>0x1F<total bytes>0x1F0x1D`,
receiver can either send an ok or not ok `ResponseHeader`, `0x1F0x000x1F0x1D`, `0x1F0x1A0x1F0x1D` respectively, sending not ok response will immediately terminate the connection. After sending an ok `ResponseHeader`, sender can start sending files.

When sending a file the sender must send a `FileHeader` which is encoded as `0x1F<size>0x1F<name>0x1F<relative-path>0x1F0x1D`, before sending the actual file bytes.
An entire file will look like `0x1F110x1Fhello_world.txt0x1F./0x1F0x1Dhello world0x1D`, sender can send files unless it explicitly sends `0x1F0x1E0x1F0x1D` which is an `EndHeader`, receiver will immediately terminate the connection.
