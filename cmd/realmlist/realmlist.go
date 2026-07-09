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
	flag.Parse()

	if flag.NArg() != 2 {
		fmt.Println("Usage: realmlist [-port N] [-timeout T] user host")
		os.Exit(1)
	}

	user := flag.Arg(0)
	host := flag.Arg(1)

	fmt.Print("Enter password: ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("")

	address := fmt.Sprintf("%v:%v", host, *PORT)
	client := wow.NewWowClient(address, user, string(password), *TIMEOUT)

	err = client.Login()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
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
