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
var ErrIpTemporarelyBlocked = errors.New("your ip is temporarely blocked")
var ErrResponseBodyBig = errors.New("response body too big")

var ErrConnectTimeout = errors.New("connect timeout")
var ErrServerMsgTimeout = errors.New("server message timeout")
var ErrTransferTimeout = errors.New("transfer timeout")

type PingResult struct {
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

var smsgAuthResponseReject = []byte{
	0, 3, // size
	238, 1, // opcode 0x1EE SMSG_AUTH_RESPONSE
	14, // result AUTH_REJECT
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
	server *Server,
	timeout time.Duration,
	respChan chan<- *PingResult,
) {
	res := &PingResult{
		Name:  server.Name,
		Group: server.Group,
	}
	startTime := time.Now()
	conn, err := net.DialTimeout("tcp", server.Address, timeout)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
			res.Error = ErrConnectTimeout
			respChan <- res
			return
		}
		res.Error = err
		respChan <- res
		return
	}
	defer conn.Close()

	connectDuration := time.Since(startTime)
	if connectDuration > timeout {
		res.Error = ErrConnectTimeout
		respChan <- res
		return
	}
	res.ConnectDuration = connectDuration

	if server.ConnectOnly {
		respChan <- res
		return
	}

	buf := make([]byte, 64)
	conn.SetDeadline(time.Now().Add(timeout))
	bytesRead, err := conn.Read(buf)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			res.Error = ErrServerMsgTimeout
			respChan <- res
			return
		}
		res.Error = err
		respChan <- res
		return
	}

	if bytesRead >= len(buf) {
		res.Error = ErrResponseBodyBig
		respChan <- res
		return
	}

	if !bytes.Equal(smsgAuthChallenge, buf[0:8]) {
		if bytes.Equal(smsgAuthResponseReject, buf[0:5]) {
			res.Error = ErrIpTemporarelyBlocked
			respChan <- res
			return
		}
		res.Error = ErrInvalidResponse
		respChan <- res
		return
	}

	conn.SetDeadline(time.Now().Add(timeout))
	writeTime := time.Now()
	_, err = conn.Write(cmsgPing)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			res.Error = ErrTransferTimeout
			respChan <- res
			return
		}
		res.Error = err
		respChan <- res
		return
	}

	buf = make([]byte, 64)
	conn.SetDeadline(time.Now().Add(timeout))
	bytesRead, err = conn.Read(buf)

	// expect the server to close connection
	if err == nil || bytesRead > 0 {
		res.Error = ErrInvalidResponse
		respChan <- res
		return
	}

	if errors.Is(err, os.ErrDeadlineExceeded) {
		res.Error = ErrTransferTimeout
		respChan <- res
		return
	}

	if !errors.Is(err, io.EOF) {
		res.Error = err
		respChan <- res
		return
	}

	respDuration := time.Since(writeTime)
	if respDuration > timeout {
		res.Error = ErrTransferTimeout
		respChan <- res
		return
	}

	res.PingDuration = respDuration
	respChan <- res
}
