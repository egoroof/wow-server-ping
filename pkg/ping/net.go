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
var ErrIpTemporarelyBlocked = errors.New("your ip is temporarely blocked")
var ErrResponseBodyBig = errors.New("response body too big")

var ErrConnectTimeout = errors.New("connect timeout")
var ErrHandshakeTimeout = errors.New("handshake timeout")
var ErrPingTimeout = errors.New("ping timeout")

type PingResult struct {
	ConnectDuration   time.Duration
	HandshakeDuration time.Duration
	PingDuration      time.Duration
	Error             error
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

var smsgAuthResponseUnknownAccount = []byte{
	0, 3, // BE size
	238, 1, // LE opcode 0x1EE SMSG_AUTH_RESPONSE
	21, // result AUTH_UNKNOWN_ACCOUNT
}

var authLogonChallengeServerUnknownAccount = []byte{
	0, // opcode AUTH_LOGON_CHALLENGE
	0, // protocol_version
	4, // result FAIL_UNKNOWN_ACCOUNT
}

func PingWowServer(
	server *Server,
	timeout time.Duration,
) *PingResult {
	res := &PingResult{}
	startTime := time.Now()
	conn, err := net.DialTimeout("tcp", server.Address, timeout)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
			res.Error = ErrConnectTimeout
			return res
		}
		res.Error = err
		return res
	}
	defer conn.Close()

	connectDuration := time.Since(startTime)
	if connectDuration > timeout {
		res.Error = ErrConnectTimeout
		return res
	}
	res.ConnectDuration = connectDuration

	if server.IsAuth {
		conn.SetDeadline(time.Now().Add(timeout))
		handshakeStartTime := time.Now()
		_, err := conn.Write(authLogonChallengeClient)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				res.Error = ErrHandshakeTimeout
				return res
			}
			res.Error = err
			return res
		}

		buf := make([]byte, 64)
		conn.SetDeadline(time.Now().Add(timeout))
		bytesRead, err := conn.Read(buf)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				res.Error = ErrHandshakeTimeout
				return res
			}
			res.Error = err
			return res
		}

		handshakeDuration := time.Since(handshakeStartTime)
		if handshakeDuration > timeout {
			res.Error = ErrHandshakeTimeout
			return res
		}
		res.HandshakeDuration = handshakeDuration

		if bytesRead >= len(buf) {
			res.Error = ErrResponseBodyBig
			return res
		}

		if !bytes.Equal(authLogonChallengeServerUnknownAccount, buf[0:3]) {
			res.Error = ErrInvalidResponse
			return res
		}

		return res
	}

	buf := make([]byte, 64)
	conn.SetDeadline(time.Now().Add(timeout))
	handshakeStartTime := time.Now()
	bytesRead, err := conn.Read(buf)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			res.Error = ErrHandshakeTimeout
			return res
		}
		res.Error = err
		return res
	}

	handshakeDuration := time.Since(handshakeStartTime)
	if handshakeDuration > timeout {
		res.Error = ErrHandshakeTimeout
		return res
	}
	res.HandshakeDuration = handshakeDuration

	if bytesRead >= len(buf) {
		res.Error = ErrResponseBodyBig
		return res
	}

	if !bytes.Equal(smsgAuthChallenge, buf[0:8]) {
		if bytes.Equal(smsgAuthResponseReject, buf[0:5]) {
			res.Error = ErrIpTemporarelyBlocked
			return res
		}
		res.Error = ErrInvalidResponse
		return res
	}

	conn.SetDeadline(time.Now().Add(timeout))
	pingStartTime := time.Now()
	_, err = conn.Write(cmsgAuthSession)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			res.Error = ErrPingTimeout
			return res
		}
		res.Error = err
		return res
	}

	buf = make([]byte, 64)
	conn.SetDeadline(time.Now().Add(timeout))
	bytesRead, err = conn.Read(buf)

	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			res.Error = ErrPingTimeout
			return res
		}

		res.Error = err
		return res
	}

	if bytesRead >= len(buf) {
		res.Error = ErrResponseBodyBig
		return res
	}

	if !bytes.Equal(smsgAuthResponseUnknownAccount, buf[0:5]) {
		res.Error = ErrInvalidResponse
		return res
	}

	pingDuration := time.Since(pingStartTime)
	if pingDuration > timeout {
		res.Error = ErrPingTimeout
		return res
	}

	res.PingDuration = pingDuration
	return res
}

var authLogonChallengeClient = []byte{
	0x0,     // Opcode CMD_AUTH_LOGON_CHALLENGE
	0x8,     // Protocol version
	30, 0x0, // LE Size
	0x57, 0x6f, 0x57, 0x0, // BE Game name: WoW\0
	0x3, 0x3, 0x5, // Version: 335
	0x34, 0x30, // LE Build: 12340
	0x36, 0x38, 0x78, 0x0, // LE Platform: \0x86
	0x6e, 0x69, 0x57, 0x0, // LE OS: \0Win
	0x55, 0x52, 0x75, 0x72, // LE Locale: ruRU
	0xe0, 0x1, 0x0, 0x0, // LE worldregion_bias: 480
	0x7f, 0x0, 0x0, 0x1, // BE Client IP: 127.0.0.1
	0, // Username byte size
}

var cmsgAuthSession = []byte{
	1, 22, // BE size
	0xed, 0x1, 0, 0, // LE opcode 0x1ED CMSG_AUTH_SESSION
	0x34, 0x30, 0, 0, // LE client_build
	0, 0, 0, 0, // LE server_id
	0,          // BE username \0
	0, 0, 0, 0, // LE login_server_type
	0, 0, 0, 0, // LE client_seed
	0, 0, 0, 0, // LE region_id
	0, 0, 0, 0, // LE battleground_id
	0, 0, 0, 0, // LE realm_id
	0, 0, 0, 0, 0, 0, 0, 0, // dos_response
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // client_proof
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
