package ping

import (
	"bytes"
	"context"
	"errors"
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

	ConnectDurationMs int
	PingDurationMs    int
	Error             error
}

var smsgAuthChallenge = []byte{
	0, 42, // BE size
	236, 1, // LE opcode 0x1EC SMSG_AUTH_CHALLENGE
	1, 0, 0, 0, // LE unknown1
	// 4x LE server_seed
	// 32x seed
}

var smsgAuthResponse = []byte{
	0, 3, // BE size
	238, 1, // LE opcode 0x1EE SMSG_AUTH_RESPONSE
	21, // result AUTH_UNKNOWN_ACCOUNT
}

// probably same?
var smsgAuthResponseEnc = []byte{125, 91, 192, 37, 13}

// Ping WoW server.
// Deals with servers behind proxy.
//
// Requests scheme behind proxy:
// 1. You -> SYN -> Proxy
// 2. Proxy -> SYN-ACK -> You
// 3. You -> ACK -> Proxy
// 4. Proxy -> SYN -> Server
// 5. Server -> SYN-ACK -> Proxy
// 6. Proxy -> ACK -> Server
// 7. Server -> smsgAuthChallenge -> Proxy -> You
// 8. You -> cmsgAuthSession -> Proxy -> Server
// 9. Server -> smsgAuthResponse -> Proxy -> You
//
// Ping
// Connect: 1 - 2
// Server : 8 - 9
//
// Timeouts (helpful for losses debug)
// Connect  : 1 - 2 (you - proxy)
// ServerMsg: 3 - 7 (proxy - server)
// Transfer : 8 - 9 (you - server)
//
// Requests scheme without proxy:
// 1. You -> SYN -> Server
// 2. Server -> SYN-ACK -> You
// 3. You -> ACK -> Server
// 4. Server -> smsgAuthChallenge -> You
// 5. You -> cmsgAuthSession -> Server
// 6. Server -> smsgAuthResponse -> You
//
// Ping
// Connect: 1 - 2
// Server : 5 - 6
//
// Timeouts (helpful for losses debug)
// Connect  : 1 - 2
// ServerMsg: 3 - 4
// Transfer : 5 - 6
func PingWowServer(
	name, group, address string,
	timeout time.Duration,
	respose chan<- ServerResponse,
) {
	startTime := time.Now()
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
			respose <- ServerResponse{
				Name:  name,
				Group: group,
				Error: ErrConnectTimeout,
			}
			return
		}
		respose <- ServerResponse{
			Name:  name,
			Group: group,
			Error: err,
		}
		return
	}
	connectDuration := time.Since(startTime)
	defer conn.Close()

	buf := make([]byte, 64)
	conn.SetDeadline(time.Now().Add(timeout))
	bytesRead, err := conn.Read(buf)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			respose <- ServerResponse{
				Name:  name,
				Group: group,
				Error: ErrServerMsgTimeout,
			}
			return
		}
		respose <- ServerResponse{
			Name:  name,
			Group: group,
			Error: err,
		}
		return
	}

	if bytesRead >= len(buf) {
		respose <- ServerResponse{
			Name:  name,
			Group: group,
			Error: ErrResponseBodyBig,
		}
		return
	}

	if !bytes.Equal(smsgAuthChallenge, buf[0:8]) {
		respose <- ServerResponse{
			Name:  name,
			Group: group,
			Error: ErrInvalidResponse,
		}
		return
	}

	conn.SetDeadline(time.Now().Add(timeout))
	writeTime := time.Now()
	_, err = conn.Write(cmsgAuthSession)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			respose <- ServerResponse{
				Name:  name,
				Group: group,
				Error: ErrTransferTimeout,
			}
			return
		}
		respose <- ServerResponse{
			Name:  name,
			Group: group,
			Error: err,
		}
		return
	}

	buf = make([]byte, 64)
	conn.SetDeadline(time.Now().Add(timeout))
	bytesRead, err = conn.Read(buf)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			respose <- ServerResponse{
				Name:  name,
				Group: group,
				Error: ErrTransferTimeout,
			}
			return
		}
		respose <- ServerResponse{
			Name:  name,
			Group: group,
			Error: err,
		}
		return
	}
	respDuration := time.Since(writeTime)

	// OS can goes sleep and deadline on read not happen
	if respDuration > timeout {
		respose <- ServerResponse{
			Name:  name,
			Group: group,
			Error: os.ErrDeadlineExceeded,
		}
		return
	}

	if bytesRead >= len(buf) {
		respose <- ServerResponse{
			Name:  name,
			Group: group,
			Error: ErrResponseBodyBig,
		}
		return
	}

	if !bytes.Equal(smsgAuthResponse, buf[0:5]) && !bytes.Equal(smsgAuthResponseEnc, buf[0:5]) {
		respose <- ServerResponse{
			Name:  name,
			Group: group,
			Error: ErrInvalidResponse,
		}
		return
	}

	respose <- ServerResponse{
		Name:              name,
		Group:             group,
		ConnectDurationMs: int(connectDuration.Milliseconds()),
		PingDurationMs:    int(respDuration.Milliseconds()),
	}
}

var cmsgAuthSession = []byte{
	1, 23, // BE size
	0xed, 0x1, 0x0, 0x0, // LE opcode 0x1ED CMSG_AUTH_SESSION
	0x34, 0x30, 0x0, 0x0, // LE client_build
	0x0, 0x0, 0x0, 0x0, // LE server_id
	0x41, 0x0, // BE username \0
	0x0, 0x0, 0x0, 0x0, // LE login_server_type
	0x9, 0x48, 0x12, 0xc6, // LE client_seed
	0x0, 0x0, 0x0, 0x0, // LE region_id
	0x0, 0x0, 0x0, 0x0, // LE battleground_id
	0x2, 0x0, 0x0, 0x0, // LE realm_id
	0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // dos_response
	0x3b, 0x75, 0x93, 0x7e, 0xed, 0xae, 0xa8, 0x4c, 0x9b, 0xe,
	0x19, 0xc, 0xea, 0x13, 0x1d, 0x26, 0x4d, 0x31, 0x75, 0xce, // client_proof
	0x9e, 0x2, 0x0, 0x0, 0x78, 0x9c, 0x75, 0xd2, 0x31, 0x6e,
	0xc3, 0x30, 0xc, 0x5, 0x50, 0xe5, 0x14, 0x59, 0x7a, 0x99,
	0x38, 0x5, 0xc, 0x3, 0xd1, 0x12, 0x2b, 0x73, 0x41, 0x4b,
	0xbf, 0x36, 0x61, 0x89, 0x32, 0x64, 0x39, 0x6d, 0x72, 0x84,
	0x9e, 0xb8, 0x63, 0xd1, 0xad, 0x5, 0xe8, 0xf9, 0x11, 0x9f,
	0xc4, 0x7, 0x8f, 0xc6, 0x98, 0x26, 0xf2, 0xf3, 0x49, 0x25,
	0xbc, 0x9d, 0xfc, 0xc4, 0xb8, 0x23, 0x41, 0xea, 0xad, 0x33,
	0x87, 0xf4, 0xf1, 0x72, 0x31, 0xff, 0xbc, 0x40, 0x48, 0x97,
	0xcd, 0x57, 0xce, 0xa2, 0x5a, 0x43, 0x65, 0x40, 0x59, 0xa7,
	0xbc, 0xec, 0x70, 0xad, 0x11, 0xef, 0x8c, 0x18, 0x2c, 0xb,
	0x27, 0x5a, 0xb4, 0x21, 0x96, 0xc0, 0x32, 0xaa, 0x1, 0x67,
	0x8a, 0x90, 0x40, 0x45, 0xa3, 0x9c, 0x6, 0xaa, 0x97, 0x3c,
	0xee, 0x9a, 0xc3, 0x67, 0x55, 0xf0, 0x15, 0xc3, 0x36, 0xba,
	0x9c, 0xe3, 0xaa, 0x60, 0x1b, 0x1f, 0xcb, 0xa4, 0x9e, 0xd2,
	0xda, 0xf3, 0x44, 0x7a, 0x77, 0xad, 0xed, 0xb7, 0x72, 0xc7,
	0x43, 0xc7, 0x8d, 0x63, 0x68, 0x48, 0x66, 0x55, 0x3b, 0x59,
	0x17, 0x78, 0x3d, 0xb6, 0xab, 0x48, 0x7d, 0xf6, 0x33, 0xea,
	0x5e, 0x3d, 0x96, 0x7c, 0xc9, 0xaa, 0x5c, 0x89, 0x83, 0xa,
	0xee, 0xb7, 0x51, 0x7d, 0x9f, 0xe3, 0x4, 0x4b, 0x42, 0x23,
	0xb4, 0xbe, 0x5d, 0x9e, 0xa1, 0x3f, 0x81, 0x2b, 0x14, 0xd0,
	0xcf, 0x1c, 0xe3, 0x1e, 0xb3, 0xa0, 0xfc, 0xb5, 0xef, 0xfe,
	0x8b, 0x7e, 0x0, 0xe3, 0xf7, 0xc9, 0x64, // addon_info
}
