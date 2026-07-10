package ping

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"time"
)

var ErrInvalidResponse = errors.New("invalid response")
var ErrResponseBodyBig = errors.New("response body too big")

var ErrConnectTimeout = errors.New("connect timeout")
var ErrServerMsgTimeout = errors.New("server message timeout")
var ErrTransferTimeout = errors.New("transfer timeout")

type ServerResponse struct {
	Name  string
	Group string

	ConnectDuration time.Duration
	PingDuration    time.Duration
	Error           error
}

var smsgAuthChallenge = []byte{
	0, 42, // BE size
	236, 1, // LE opcode 0x1EC SMSG_AUTH_CHALLENGE
	1, 0, 0, 0, // LE unknown1
	// 4x LE server_seed
	// 32x seed
}

var cmsgPing = []byte{
	0, 12, // BE size
	220, 1, 0, 0, // LE opcode 0x1DC CMSG_PING
	0, 0, 0, 0, // LE sequence_id
	0, 0, 0, 0, // LE latency
}

// Ping WoW server.
// Deals with servers behind proxy.
func PingWowServer(
	name, group, address string,
	timeout time.Duration,
	respChan chan<- ServerResponse,
) {
	resp := ServerResponse{
		Name:  name,
		Group: group,
	}
	startTime := time.Now()
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
			resp.Error = ErrConnectTimeout
			respChan <- resp
			return
		}
		resp.Error = err
		respChan <- resp
		return
	}
	defer conn.Close()

	connectDuration := time.Since(startTime)
	if connectDuration > timeout {
		resp.Error = ErrConnectTimeout
		respChan <- resp
		return
	}
	resp.ConnectDuration = connectDuration

	buf := make([]byte, 64)
	conn.SetDeadline(time.Now().Add(timeout))
	bytesRead, err := conn.Read(buf)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			resp.Error = ErrServerMsgTimeout
			respChan <- resp
			return
		}
		resp.Error = err
		respChan <- resp
		return
	}

	if bytesRead >= len(buf) {
		resp.Error = ErrResponseBodyBig
		respChan <- resp
		return
	}

	if !bytes.Equal(smsgAuthChallenge, buf[0:8]) {
		resp.Error = ErrInvalidResponse
		respChan <- resp
		return
	}

	conn.SetDeadline(time.Now().Add(timeout))
	writeTime := time.Now()
	_, err = conn.Write(cmsgPing)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			resp.Error = ErrTransferTimeout
			respChan <- resp
			return
		}
		resp.Error = err
		respChan <- resp
		return
	}

	buf = make([]byte, 64)
	conn.SetDeadline(time.Now().Add(timeout))
	bytesRead, err = conn.Read(buf)

	// expect the server to close connection
	if err == nil || bytesRead > 0 {
		resp.Error = ErrInvalidResponse
		respChan <- resp
		return
	}

	if errors.Is(err, os.ErrDeadlineExceeded) {
		resp.Error = ErrTransferTimeout
		respChan <- resp
		return
	}

	if !errors.Is(err, io.EOF) {
		resp.Error = err
		respChan <- resp
		return
	}

	respDuration := time.Since(writeTime)
	if respDuration > timeout {
		resp.Error = ErrTransferTimeout
		respChan <- resp
		return
	}

	resp.PingDuration = respDuration
	respChan <- resp
}
