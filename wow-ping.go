package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/egoroof/wow-server-ping/pkg/ping"
	"github.com/egoroof/wow-server-ping/pkg/prometheus"
)

var LISTEN_PORT = flag.Int("port", 0, "listen port for Prometheus metrics")
var PING_INTERVAL = flag.Duration("interval", time.Second, "sleep time between requests")
var PING_TIMEOUT = flag.Duration("timeout", time.Second, "ping timeout")
var STATS_INTERVAL = flag.Duration("stats-interval", time.Second*10, "how often stats should be printed to console")
var STATS_COUNT = flag.Int("stats", 0, "how many stats to display before exit")
var FILTER = flag.String("filter", "", "regexp for filter servers by name")

func recordMetrics(servers []ping.Server, stats *ping.Store, prom *prometheus.ResponseMetrics) {
	responseChan := make(chan ping.ServerResponse)

	stats.Init(servers)
	if prom != nil {
		prom.Init(servers)
	}

	statsLogTime := time.Now()
	statsCount := 0
	requestCount := 0
	for {
		requestCount++
		for _, server := range servers {
			go ping.PingWowServer(
				server.Name, server.Group, server.Address, *PING_TIMEOUT, responseChan,
			)
		}

		for range servers {
			resp := <-responseChan

			stats.Update(resp)
			if prom != nil {
				prom.Update(resp)
			}
		}

		if time.Now().After(statsLogTime.Add(*STATS_INTERVAL)) {
			fmt.Printf(
				"\n%v to %v sent %v requests\n",
				statsLogTime.Format(time.TimeOnly),
				time.Now().Format(time.TimeOnly),
				requestCount,
			)
			stats.Print()
			stats.Reset()
			statsLogTime = time.Now()
			statsCount++
			requestCount = 0

			if *STATS_COUNT == statsCount {
				fmt.Println("Exiting due to stats count flag is set and reached")
				os.Exit(0)
			}
		}

		time.Sleep(*PING_INTERVAL)
	}
}

func main() {
	fmt.Println("Ping tool for World of Warcraft 3.3.5a servers.")
	flag.Parse()

	configsWithComma := flag.Arg(0)

	if configsWithComma == "" {
		fmt.Print("Enter config names: ")
		_, err := fmt.Scanln(&configsWithComma)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	configs := strings.Split(configsWithComma, ",")

	fmt.Printf("Ping timeout: %v\n", *PING_TIMEOUT)
	fmt.Printf("Ping interval: %v\n", *PING_INTERVAL)
	fmt.Printf("Stats interval: %v\n", *STATS_INTERVAL)

	if *STATS_COUNT != 0 {
		fmt.Printf("Stats count: %v\n", *STATS_COUNT)
	}

	if *LISTEN_PORT == 0 {
		fmt.Println("Listen port is not set. Prometheus metrics disabled")
	} else {
		fmt.Printf("Listen port: %v\n", *LISTEN_PORT)
	}

	var filter *regexp.Regexp
	if *FILTER != "" {
		fmt.Printf("Realm name filter: \"%v\"\n", *FILTER)
		filter = regexp.MustCompile(*FILTER)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	var allServers []ping.Server
	for _, configName := range configs {
		serversPath := fmt.Sprintf("./servers/%v.json", configName)
		fmt.Printf("\nRealm list %v\n", serversPath)

		serversFile, err := os.ReadFile(serversPath)
		if err != nil {
			fmt.Println("Error when opening file: ", err)
			os.Exit(1)
		}

		var servers []ping.Server
		err = json.Unmarshal(serversFile, &servers)
		if err != nil {
			fmt.Println("Error during Unmarshal(): ", err)
			os.Exit(1)
		}

		for _, server := range servers {
			if *FILTER != "" && !filter.MatchString(server.Name) {
				continue
			}

			fmt.Fprintf(w, "%v\t%v\n", server.Name, server.Address)

			server.Group = configName
			allServers = append(allServers, server)
		}
		w.Flush()
	}

	if len(allServers) == 0 {
		fmt.Println("No realms found")
		os.Exit(1)
	}

	stats := ping.NewStatsStore(configsWithComma)
	if *LISTEN_PORT == 0 {
		recordMetrics(allServers, stats, nil)
	} else {
		prom := prometheus.NewResponseMetrics()
		go recordMetrics(allServers, stats, prom)
		err := prom.ListenAndServe(*LISTEN_PORT)
		fmt.Println(err)
	}
}
