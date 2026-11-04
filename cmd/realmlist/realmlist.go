package main

import (
	"bufio"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"slices"
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

	if config == "" {
		fmt.Print("Enter config name (where to save result): ")
		config = readLine()
	} else {
		fmt.Printf("Config: %v\n", config)
	}

	filename := fmt.Sprintf("./servers/%v.json", config)
	oldConfig, err := readConfig(filename)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if len(oldConfig.RealmList) > 0 {
		fmt.Print("Config already exist. Overwrite? (y/N) ")

		if readLine() != "y" {
			fmt.Println("You should enter 'y' to overwite config or choose another config name.")
			os.Exit(0)
		}
	}

	hostPortList := []string{}
	enterRealmlist := false

	if len(oldConfig.RealmList) > 0 {
		fmt.Printf("Use existing realmlist %v? (y/N) ", strings.Join(oldConfig.RealmList, ", "))

		if readLine() == "y" {
			hostPortList = oldConfig.RealmList
		} else {
			enterRealmlist = true
		}
	} else {
		enterRealmlist = true
	}

	if enterRealmlist {
		fmt.Print("Enter realmlist: ")
		hostPortList = strings.Split(readLine(), ",")
		for i, elem := range hostPortList {
			hostPortList[i] = strings.TrimSpace(elem)
		}
	}

	var addressList []string
	for _, hostPort := range hostPortList {
		host, port, err := splitHostPort(hostPort, defaultAuthPort)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Printf("Resolving %v\n", host)
		ips, err := resolver.LookupHost(host, *TIMEOUT)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Printf("Resolved: %v\n", strings.Join(ips, ", "))

		for _, ip := range ips {
			address := fmt.Sprintf("%v:%v", ip, port)
			addressList = append(addressList, address)
		}
	}

	fmt.Print("Enter username: ")
	user := readLine()

	fmt.Print("Enter password: ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("")

	randomAddress := addressList[rand.IntN(len(addressList))]
	client := wow.NewWowClient(randomAddress, user, string(password), *TIMEOUT)

	fmt.Printf("Connecting to %v\n", randomAddress)
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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Loaded %v realms\n\nName\tAddress\n", len(realms))
	for _, realm := range realms {
		fmt.Fprintf(w, "%v\t%v\n", realm.Name, realm.Address)
	}
	fmt.Fprintf(w, "\n")
	w.Flush()

	configChanged := false

	if len(oldConfig.RealmList) > 0 && !slices.Equal(oldConfig.RealmList, hostPortList) {
		configChanged = true
		fmt.Printf(
			"RealmList updated. Was %v, now %v\n",
			strings.Join(oldConfig.RealmList, ", "),
			strings.Join(hostPortList, ", "),
		)
	}

	var configRealms []ping.Realm
	for _, realm := range realms {
		foundInOldConfig := false
		for _, oldRealm := range oldConfig.Realms {
			// todo compare by ID ?
			if realm.Name == oldRealm.Name {
				// same, copy ShortName and ProxyName
				configRealms = append(configRealms, ping.Realm{
					Name:      realm.Name,
					Address:   realm.Address,
					ShortName: oldRealm.ShortName,
					ProxyName: oldRealm.ProxyName,
				})
				if realm.Address != oldRealm.Address {
					configChanged = true
					fmt.Printf("- update address for %v. Was %v, now %v\n",
						realm.Name, oldRealm.Address, realm.Address,
					)
				}
				foundInOldConfig = true
				break
			}
		}
		if !foundInOldConfig {
			// new realm
			configChanged = true
			configRealms = append(configRealms, ping.Realm{
				Name:      realm.Name,
				Address:   realm.Address,
				ShortName: realm.Name,
				ProxyName: "Main",
			})

			if len(oldConfig.Realms) > 0 {
				fmt.Printf("+ new realm %v\n", realm.Name)
			}
		}
	}

	// checking for deleted realms
	for _, oldRealm := range oldConfig.Realms {
		foundInNewConfig := false
		for _, realm := range realms {
			// todo compare by ID ?
			if realm.Name == oldRealm.Name {
				foundInNewConfig = true
				break
			}
		}
		if !foundInNewConfig {
			configChanged = true
			fmt.Printf("- delete old realm %v, address %v, shortName %v, proxy %v\n",
				oldRealm.Name, oldRealm.Address, oldRealm.ShortName, oldRealm.ProxyName,
			)
		}
	}

	if !configChanged {
		fmt.Println("No changes to config")
		os.Exit(0)
	}

	serverConfig := ping.ServerConfig{
		RealmList: hostPortList,
		Realms:    configRealms,
	}
	err = writeConfig(filename, &serverConfig)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Saved to %v\n", filename)
}

func splitHostPort(hostport string, fallbackPort string) (string, string, error) {
	if !strings.Contains(hostport, ":") {
		return hostport, fallbackPort, nil
	}

	return net.SplitHostPort(hostport)
}

func readLine() string {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return scanner.Text()
}

func writeConfig(filename string, config *ping.ServerConfig) error {
	json, err := json.Marshal(config, jsontext.Multiline(true))
	if err != nil {
		return err
	}

	err = os.WriteFile(filename, json, 0644)
	if err != nil {
		return err
	}

	return nil
}

func readConfig(filename string) (*ping.ServerConfig, error) {
	var config ping.ServerConfig

	configFile, err := os.ReadFile(filename)
	if errors.Is(err, os.ErrNotExist) {
		return &config, nil
	}
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(configFile, &config)
	return &config, err
}
