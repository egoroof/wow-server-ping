package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/egoroof/wow-server-ping/pkg/wow"
	"golang.org/x/term"
)

var PORT = flag.Int("port", 3724, "realmlist server port")
var TIMEOUT = flag.Duration("timeout", time.Second*10, "timeout for network operations")

func main() {
	fmt.Println("World of Warcraft 3.3.5a realm list extractor.")
	flag.Parse()

	host := flag.Arg(0)
	user := flag.Arg(1)

	if host == "" {
		fmt.Print("Enter host: ")
		_, err := fmt.Scanln(&host)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		fmt.Printf("Host: %v\n", host)
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

	address := fmt.Sprintf("%v:%v", host, *PORT)
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

	filename := fmt.Sprintf("./servers/%v.json", host)
	json, err := json.MarshalIndent(realms, "", "	")
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
