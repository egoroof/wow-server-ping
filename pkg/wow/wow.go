package wow

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/egoroof/wow-server-ping/pkg/srp6"
)

// based on https://wowdev.wiki/Login_Packet

var Err2faRequired = errors.New("2fa required")

type wowClient struct {
	address string
	conn    net.Conn
	timeout time.Duration

	username string
	password string

	serverPublicKey []byte
	salt            []byte
	securityFlag    byte

	clientPublicKey  []byte
	clientPrivateKey []byte
	clientSessionKey []byte
	clientProof      []byte

	realms []realm
}

func NewWowClient(
	address, username, password string,
	timeout time.Duration,
) *wowClient {
	return &wowClient{
		address:  address,
		username: username,
		password: password,
		timeout:  timeout,
	}
}

func (w *wowClient) Login(authenticator string) error {
	conn, err := net.DialTimeout("tcp", w.address, w.timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	w.conn = conn

	err = w.writeAuthLogonChallengeClient()
	if err != nil {
		return fmt.Errorf("writeAuthLogonChallengeClient error: %w", err)
	}

	err = w.readAuthLogonChallengeServer()
	if err != nil {
		if errors.Is(err, ErrLoginFailed) {
			return err
		} else {
			return fmt.Errorf("readAuthLogonChallengeServer error: %w", err)
		}
	}

	if w.securityFlag != 0 && w.securityFlag != 4 {
		return fmt.Errorf("2fa type AUTHENTICATOR supported only")
	}
	if w.securityFlag == 4 && authenticator == "" {
		return Err2faRequired
	}

	w.clientPrivateKey = srp6.ClientPrivateKey()
	w.clientPublicKey = srp6.ClientPublicKey(w.clientPrivateKey)
	w.clientSessionKey = srp6.ClientSessionKey(
		w.username, w.password, w.salt, w.clientPublicKey, w.clientPrivateKey, w.serverPublicKey,
	)
	w.clientProof = srp6.ClientProof(
		w.username, w.salt, w.clientPublicKey, w.serverPublicKey, w.clientSessionKey,
	)

	err = w.writeAuthLogonProofClient(authenticator)
	if err != nil {
		return fmt.Errorf("writeAuthLogonProofClient error: %w", err)
	}

	err = w.readAuthLogonProofServer()
	if err != nil {
		if errors.Is(err, ErrLoginFailed) {
			return err
		} else {
			return fmt.Errorf("readAuthLogonProofServer error: %w", err)
		}
	}

	err = w.writeRealmListClient()
	if err != nil {
		return fmt.Errorf("writeRealmListClient error: %w", err)
	}

	err = w.readRealmListServer()
	if err != nil {
		return fmt.Errorf("readRealmListServer error: %w", err)
	}

	return nil
}

func (w *wowClient) GetRealmList() []realm {
	return w.realms
}

func (w *wowClient) writeAuthLogonChallengeClient() error {
	cmd := []byte{
		0x0,                             // Opcode CMD_AUTH_LOGON_CHALLENGE
		0x8,                             // Protocol version
		byte(30 + len(w.username)), 0x0, // LE Size
		0x57, 0x6f, 0x57, 0x0, // BE Game name: WoW\0
		0x3, 0x3, 0x5, // Version: 335
		0x34, 0x30, // LE Build: 12340
		0x36, 0x38, 0x78, 0x0, // LE Platform: \0x86
		0x6e, 0x69, 0x57, 0x0, // LE OS: \0Win
		0x55, 0x52, 0x75, 0x72, // LE Locale: ruRU
		0xe0, 0x1, 0x0, 0x0, // LE worldregion_bias: 480
		0x7f, 0x0, 0x0, 0x1, // BE Client IP: 127.0.0.1
		byte(len(w.username)), // Username byte size
	}
	cmd = append(cmd, strings.ToUpper(w.username)...)
	w.conn.SetWriteDeadline(time.Now().Add(w.timeout))
	_, err := w.conn.Write(cmd)
	if err != nil {
		return err
	}
	return nil
}

func (w *wowClient) writeAuthLogonProofClient(authenticator string) error {
	cmd := []byte{
		0x1, // Opcode CMD_AUTH_LOGON_PROOF
	}
	cmd = append(cmd, w.clientPublicKey...)
	cmd = append(cmd, w.clientProof...)
	// crc_hash (20 bytes) + num_keys
	cmd = append(cmd, make([]byte, 21)...)
	cmd = append(cmd, w.securityFlag)

	if w.securityFlag == 4 {
		cmd = append(cmd, byte(len(authenticator)))
		cmd = append(cmd, authenticator...)
	}

	w.conn.SetWriteDeadline(time.Now().Add(w.timeout))
	_, err := w.conn.Write(cmd)
	if err != nil {
		return err
	}
	return nil
}

func (w *wowClient) writeRealmListClient() error {
	cmd := []byte{0x10, 0, 0, 0, 0}
	w.conn.SetWriteDeadline(time.Now().Add(w.timeout))
	_, err := w.conn.Write(cmd)
	if err != nil {
		return err
	}
	return nil
}

func (w *wowClient) readAuthLogonChallengeServer() error {
	buf := make([]byte, 256)
	w.conn.SetReadDeadline(time.Now().Add(w.timeout))
	bytesRead, err := w.conn.Read(buf)
	if err != nil {
		return err
	}

	opcode := buf[0]
	cursor := 1

	protocolVersion := buf[cursor]
	cursor++

	if opcode != 0 || protocolVersion != 0 {
		return fmt.Errorf("invalid header")
	}
	if bytesRead >= len(buf) {
		return fmt.Errorf("body too big")
	}

	result := buf[cursor]
	cursor++
	if result != 0 {
		return fmt.Errorf("%w: %v", ErrLoginFailed, loginResultMessage[result])
	}

	w.serverPublicKey = buf[cursor : cursor+32]
	cursor += 32

	generatorLen := buf[cursor]
	cursor += 1

	if generatorLen != 1 {
		return fmt.Errorf("invalid generatorLen")
	}

	generator := buf[cursor]
	cursor += 1

	if generator != 7 {
		return fmt.Errorf("invalid generator")
	}

	largeSafePrimeLen := buf[cursor]
	cursor += 1

	if largeSafePrimeLen != 32 {
		return fmt.Errorf("invalid largeSafePrimeLen")
	}

	largeSafePrime := buf[cursor : cursor+32]
	cursor += 32

	if !bytes.Equal(largeSafePrime, srp6.LargeSafePrime) {
		return fmt.Errorf("invalid largeSafePrime")
	}

	w.salt = buf[cursor : cursor+32]
	cursor += 32

	cursor += 16 // crc_salt

	w.securityFlag = buf[cursor]
	cursor++

	return nil
}

func (w *wowClient) readAuthLogonProofServer() error {
	buf := make([]byte, 256)
	w.conn.SetReadDeadline(time.Now().Add(w.timeout))
	bytesRead, err := w.conn.Read(buf)
	if err != nil {
		return err
	}

	opcode := buf[0]
	cursor := 1
	if opcode != 1 {
		return fmt.Errorf("invalid header")
	}
	if bytesRead >= len(buf) {
		return fmt.Errorf("body too big")
	}

	result := buf[cursor]
	cursor++
	if result != 0 {
		return fmt.Errorf("%w: %v", ErrLoginFailed, loginResultMessage[result])
	}
	return nil
}

func (w *wowClient) readRealmListServer() error {
	buf := make([]byte, 4096)
	w.conn.SetReadDeadline(time.Now().Add(w.timeout))
	bytesRead, err := w.conn.Read(buf)
	if err != nil {
		return err
	}

	opcode := buf[0]
	cursor := 1

	if opcode != 0x10 {
		return fmt.Errorf("invalid header")
	}
	if bytesRead >= len(buf) {
		return fmt.Errorf("body too big")
	}

	cursor += 2 // size
	cursor += 4 // padding

	numRealms := buf[cursor]
	cursor += 2 // 255 realms should be enough? skip second byte

	w.realms = make([]realm, 0, numRealms)

	for range numRealms {
		realmType := buf[cursor]
		cursor++

		locked := buf[cursor]
		cursor++

		flag := buf[cursor]
		cursor++

		var name strings.Builder
		char := buf[cursor]
		cursor++
		for char != 0 {
			name.WriteByte(char)
			char = buf[cursor]
			cursor++
		}

		var address strings.Builder
		char = buf[cursor]
		cursor++
		for char != 0 {
			address.WriteByte(char)
			char = buf[cursor]
			cursor++
		}

		population := buf[cursor : cursor+4]
		cursor += 4

		numChars := buf[cursor]
		cursor++

		category := buf[cursor]
		cursor++

		realmId := buf[cursor]
		cursor++

		realm := realm{
			realmType:  realmType,
			locked:     locked,
			flag:       flag,
			Name:       name.String(),
			Address:    address.String(),
			population: population,
			numChars:   numChars,
			category:   category,
			realmId:    realmId,
		}

		w.realms = append(w.realms, realm)
	}

	slices.SortFunc(w.realms, func(a, b realm) int {
		return strings.Compare(a.Name, b.Name)
	})

	return nil
}
