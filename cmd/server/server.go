package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/egoroof/wow-server-ping/pkg/srp6"
)

// mock server for testing

const AUTH_PORT = 3724
const WORLD_PORT = 8085

func main() {
	auth, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%v", AUTH_PORT))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Auth listen %v\n", auth.Addr())

	world, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%v", WORLD_PORT))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("World listen %v\n", world.Addr())

	authConnChan := make(chan bool)
	worldConnChan := make(chan bool)
	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go acceptAuthConnection(auth, authConnChan)
	go acceptWorldConnection(world, worldConnChan)

	for {
		select {
		case <-authConnChan:
			go acceptAuthConnection(auth, authConnChan)
		case <-worldConnChan:
			go acceptWorldConnection(world, worldConnChan)
		case <-sigChan:
			fmt.Println("Exiting")
			os.Exit(0)
		}
	}
}

func acceptAuthConnection(listener net.Listener, accepted chan<- bool) {
	conn, err := listener.Accept()
	if err != nil {
		fmt.Println(err)
		accepted <- true
		return
	}
	fmt.Printf("Auth new connection from %v\n", conn.RemoteAddr())
	go handleAuthConnection(conn)
	accepted <- true
}

func handleAuthConnection(conn net.Conn) {
	defer conn.Close()

	// AuthLogonChallengeClient
	buf := make([]byte, 256)
	_, err := conn.Read(buf)
	if err != nil {
		fmt.Println(err)
		return
	}

	// AuthLogonChallengeServer
	// opcode, protocol, result
	cmd := []byte{0, 0, 0}

	// serverPublicKey 32
	cmd = append(cmd, make([]byte, 32)...)

	// generatorLen, generator, largeSafePrimeLen
	cmd = append(cmd, []byte{1, 7, 32}...)

	// largeSafePrime 32
	cmd = append(cmd, srp6.LargeSafePrime...)

	// salt 32, crc_salt 16, securityFlag
	cmd = append(cmd, make([]byte, 32+16+1)...)

	_, err = conn.Write(cmd)
	if err != nil {
		fmt.Println(err)
	}

	// AuthLogonProofClient
	buf = make([]byte, 256)
	_, err = conn.Read(buf)
	if err != nil {
		fmt.Println(err)
		return
	}

	// AuthLogonProofServer
	cmd = []byte{1, 0}
	_, err = conn.Write(cmd)
	if err != nil {
		fmt.Println(err)
	}

	// RealmListClient
	buf = make([]byte, 256)
	_, err = conn.Read(buf)
	if err != nil {
		fmt.Println(err)
		return
	}

	// RealmListServer
	cmd = []byte{
		0x10, // opcode
		0, 0, // size
		0, 0, 0, 0, // padding
		1, 0, // num_realms
		0, // realmType
		0, // locked
		0, // flag

		84, 114, 105, 110, 105, 116, 121, 0, // name Trinity \0
		49, 50, 55, 46, 48, 46, 48, 46, 49, 58, 56, 48, 56, 53, 0, // address 127.0.0.1:8085 \0
		0, 0, 0, 0, // population
		0, // numChars
		0, // category
		0, // realmId
	}
	_, err = conn.Write(cmd)
	if err != nil {
		fmt.Println(err)
	}
}

func acceptWorldConnection(listener net.Listener, accepted chan<- bool) {
	conn, err := listener.Accept()
	if err != nil {
		fmt.Println(err)
		accepted <- true
		return
	}
	fmt.Printf("World new connection from %v\n", conn.RemoteAddr())
	go handleWorldConnection(conn)
	accepted <- true
}

func handleWorldConnection(conn net.Conn) {
	defer conn.Close()

	cmd := []byte{
		0, 42, // BE size
		236, 1, // LE opcode 0x1EC SMSG_AUTH_CHALLENGE
		1, 0, 0, 0, // LE unknown1
	}
	// server_seed 4, seed 32
	cmd = append(cmd, make([]byte, 4+32)...)
	_, err := conn.Write(cmd)
	if err != nil {
		fmt.Println(err)
		return
	}

	buf := make([]byte, 256)
	_, err = conn.Read(buf)
	if err != nil {
		fmt.Println(err)
		return
	}
}
