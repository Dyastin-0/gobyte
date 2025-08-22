package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Dyastin-0/gobyte/progress"
	"github.com/Dyastin-0/gobyte/styles"
	"github.com/Dyastin-0/gobyte/tofu"
	"github.com/Dyastin-0/gobyte/types"
)

func (c *Client) StartChompListener(
	ctx context.Context,
	dir string,
	onNewPeer func(string, string) bool,
	onRequest func(msg types.Message) (bool, error),
) error {
	go c.presenceBroadcaster(ctx)

	addr := fmt.Sprintf(":%d", c.transferPort)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		c.logger.Error(err.Error())
		homeDir = "."
	}
	certDir := fmt.Sprintf("%s/gobyte/cert", homeDir)
	trustDir := fmt.Sprintf("%s/gobyte/trust", homeDir)

	tofuID := fmt.Sprintf("%s (%s)", c.Self.Name, c.Self.IPAddress)

	tofu, err := tofu.New(tofuID, certDir, trustDir)
	if err != nil {
		c.logger.Error(err.Error())
		return err
	}

	tofu.OnNewPeer = onNewPeer

	handler := func(listener net.Listener) error {
		for {
			select {
			case <-ctx.Done():
				c.logger.Info(ctx.Err().Error())
				listener.Close()
				return ctx.Err()

			case msg := <-c.transferReqChan:
				fmt.Println(
					styles.TITLE.Render(
						fmt.Sprintf("chuck request from %s (%s)", msg.SenderName, msg.IPAddress),
					),
				)

				confirm, err := onRequest(msg)
				if err != nil {
					fmt.Println(styles.ERROR.Render())
					c.logger.Error(err.Error())
					return fmt.Errorf("request from %s timed out", msg.SenderName)
				}

				err = c.sendAck(msg, "", confirm)
				if err != nil {
					c.logger.Error(err.Error())
					return err
				}

				if !confirm {
					c.logger.Info("request rejected")
					return fmt.Errorf("request rejected")
				}

				conn, err := listener.Accept()
				if err != nil {
					c.logger.Error(err.Error())
					return fmt.Errorf("failed to accept connection: %v", err)
				}

				conn = conn.(*tls.Conn)
				conn.SetWriteDeadline(time.Now().Add(15 * time.Second))

				_, err = conn.Write([]byte("OK"))
				if err != nil {
					c.logger.Error(err.Error())
					return err
				}

				if err := c.readFiles(conn, dir); err != nil {
					c.logger.Error(err.Error())
					return fmt.Errorf("failed to chomp: %v", err)
				}

				fmt.Println(styles.SUCCESS.Bold(true).Render("all files chomped ✓"))
				c.Busy = false
			}
		}
	}

	return tofu.Start(addr, handler)
}

func (c *Client) sendAck(msg types.Message, reason string, confirm bool) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", msg.IPAddress, c.discoveryPort))
	if err != nil {
		return fmt.Errorf("failed to resolve udp addr: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to dial udp: %v", err)
	}

	defer conn.Close()

	ack := types.Message{
		Type:       types.TypeTransferAck,
		SenderID:   c.Self.ID,
		SenderName: c.Self.Name,
		IPAddress:  c.Self.IPAddress,
		Accepted:   confirm,
		Reason:     reason,
		TransferID: msg.TransferID,
	}

	ackBytes, err := json.Marshal(ack)
	if err != nil {
		return fmt.Errorf("failed to marshal ackMessage: %v", err)
	}

	_, err = conn.Write(ackBytes)
	if err != nil {
		return fmt.Errorf("failed to write ackMessage: %v", err)
	}

	return nil
}

func (c *Client) readFiles(conn net.Conn, dir string) error {
	defer conn.Close()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	reader := bufio.NewReader(conn)

	for {
		header, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				c.logger.Error(err.Error())
				break
			}

			if err == tofu.ErrorConnectionDenied {
				c.logger.Error(err.Error())
				return err
			}

			fmt.Println(styles.ERROR.Render(fmt.Sprintf("error reading file header: %v", err)))

			c.logger.Error(err.Error())
			break
		}

		header = strings.TrimSpace(header)
		if header == "END" {
			c.logger.Error("END")
			break
		}

		fileName, fileSize, err := readFileHeader(header)
		if err != nil {
			fmt.Println(styles.ERROR.Render(err.Error()))
			c.logger.Error(err.Error())
			continue
		}

		_, err = writeBytesToDir(reader, fileSize, dir, fileName)
		if err != nil {
			fmt.Println(styles.ERROR.Render(err.Error()))
			c.logger.Error(err.Error())
			continue
		}
	}

	return nil
}

func writeBytesToDir(reader io.Reader, fileSize int64, dir, fileName string) (int64, error) {
	filePath := filepath.Join(dir, fileName)

	if _, err := os.Stat(filePath); err == nil {
		base := filepath.Base(fileName)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		filePath = filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, time.Now().Unix(), ext))
	}

	file, err := os.Create(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to create %s: %v", filePath, err)
	}

	p := progress.New()

	defer func() {
		file.Close()
		p.Reset()
	}()

	bar := p.NewBar(fileSize, fmt.Sprintf("chomping %s...", fileName))

	proxyReader := bar.ProxyReader(reader)
	defer proxyReader.Close()

	copiedBytes, err := io.CopyN(file, proxyReader, fileSize)
	if err != nil {
		return 0, fmt.Errorf("failed to copy %s: %v", file.Name(), err)
	}

	p.Wait()

	return copiedBytes, nil
}

func readFileHeader(header string) (string, int64, error) {
	parts := strings.Split(header, ":")
	if len(parts) != 3 {
		return "", 0, fmt.Errorf("invalid file header: %s", header)
	}

	if !strings.HasPrefix(header, "FILE:") {
		return "", 0, fmt.Errorf("invalid header: %s", header)
	}

	fileName := parts[1]

	fileSize, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("%s has invalid size: %d", fileName, fileSize)
	}

	return fileName, fileSize, nil
}
