package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand/v2"
	"net"
	"net/netip"
	"os"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/egoroof/wow-server-ping/pkg/ping"
	"github.com/egoroof/wow-server-ping/pkg/resolver"
	"github.com/egoroof/wow-server-ping/pkg/wow"
	"golang.org/x/term"
)

var TIMEOUT = flag.Duration("timeout", time.Second*10, "timeout for network operations")

const defaultAuthPort = "3724"

func main() {
	fmt.Println("World of Warcraft 3.3.5a realm list extractor.")
	flag.Parse()

	config := flag.Arg(0)
	hostPort := flag.Arg(1)
	user := flag.Arg(2)

	if config == "" {
		fmt.Print("Enter config name (where to save result): ")
		_, err := fmt.Scanln(&config)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		fmt.Printf("Config: %v\n", config)
	}

	if hostPort == "" {
		fmt.Print("Enter host: ")
		_, err := fmt.Scanln(&hostPort)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		fmt.Printf("Host: %v\n", hostPort)
	}

	var err error
	host := hostPort
	port := defaultAuthPort
	if strings.Contains(hostPort, ":") {
		host, port, err = net.SplitHostPort(hostPort)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	if user == "" {
		fmt.Print("Enter username: ")
		_, err := fmt.Scanln(&user)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		fmt.Printf("Username: %v\n", user)
	}

	fmt.Print("Enter password: ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("")

	var ips []string
	if _, err := netip.ParseAddr(host); err == nil {
		// host is IP
		ips = []string{host}
	} else {
		fmt.Printf("Resolving %v\n", host)
		ips, err = resolver.LookupHost(host, *TIMEOUT)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Printf("Resolved: %v\n", strings.Join(ips, ", "))
	}

	var addressList []string
	for _, ip := range ips {
		addressList = append(addressList, fmt.Sprintf("%v:%v", ip, port))
	}

	address := addressList[rand.IntN(len(addressList))]
	client := wow.NewWowClient(address, user, string(password), *TIMEOUT)

	err = client.Login("")
	if err != nil {
		if errors.Is(err, wow.Err2faRequired) {
			fmt.Print("Enter authenticator code: ")
			authenticator := ""
			_, err := fmt.Scanln(&authenticator)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			err = client.Login(authenticator)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		} else {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	realms := client.GetRealmList()

	if len(realms) == 0 {
		fmt.Println("Server has 0 realms")
		os.Exit(1)
	}

	fmt.Printf("Loaded %v realms\n", len(realms))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "\nName\tAddress\n")
	for _, realm := range realms {
		fmt.Fprintf(w, "%v\t%v\n", realm.Name, realm.Address)
	}
	w.Flush()
	fmt.Println("")

	serverConfig := ping.ServerConfig{
		Host:        host,
		Port:        port,
		AddressList: addressList,
		Realms:      realms,
	}
	filename := fmt.Sprintf("./servers/%v.json", config)
	json, err := json.MarshalIndent(serverConfig, "", "	")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	oldFile, err := os.ReadFile(filename)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Println(err)
		os.Exit(1)
	}
	if bytes.Equal(oldFile, json) {
		fmt.Printf("File %v has the same realm list\n", filename)
		os.Exit(0)
	}
	err = os.WriteFile(filename, json, 0644)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Saved to %v\n", filename)
}
