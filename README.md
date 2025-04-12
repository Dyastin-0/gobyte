# gobyte

A local area network file sharing CLI app. 

## Features

- Automatic peer discovery and disconnection
- Multi-peer selection
- Multi-file selection
- File tree navigation

## Basic Usage

### Sender

Set the `-d` or `-dir` flag to the initial directory you want to start on when selecting files.

```
gobyte chuck -d ~/Documents
```

### Receiver

Set the `-d` or `-dir` flag to the directory you want to receive files to, default is set to `~/gobyte/received`.

```
gobyte chomp -d ~/Documents/gobyte/received
```
