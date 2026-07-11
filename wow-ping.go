package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/egoroof/wow-server-ping/pkg/ping"
)

var LISTEN_PORT = flag.Int("port", 0, "listen port for Prometheus metrics")
var PING_INTERVAL = flag.Duration("interval", time.Second, "sleep time between requests")
var PING_TIMEOUT = flag.Duration("timeout", time.Second, "ping timeout")
var STATS_INTERVAL = flag.Duration("stats-interval", time.Second*10, "how often stats should be printed to console")
var STATS_COUNT = flag.Int("stats", 0, "how many stats to display before exit")
var FILTER = flag.String("filter", "", "regexp for filter servers by name")

var promRespTime = ping.PrometheusMetric{
	Name:       "wow_server_response_time_ms",
	Help:       "WoW server response time in ms",
	Type:       "gauge",
	LabelNames: []string{"server", "group"},
}
var promConnTime = ping.PrometheusMetric{
	Name:       "wow_server_connect_time_ms",
	Help:       "WoW server connect time in ms",
	Type:       "gauge",
	LabelNames: []string{"server", "group"},
}
var promRespTimeout = ping.PrometheusMetric{
	Name:       "wow_server_timeout_count",
	Help:       "WoW server timeout count",
	Type:       "counter",
	LabelNames: []string{"server", "group", "type"},
}
var promRespErr = ping.PrometheusMetric{
	Name:       "wow_server_error_count",
	Help:       "WoW server error count",
	Type:       "counter",
	LabelNames: []string{"server", "group"},
}

const promTypeConnectTimeout = "connect"
const promTypeServerMsgTimeout = "serverMsg"
const promTypeTransferTimeout = "transfer"

func recordMetrics(servers []ping.Server, statsGroupOrder string) {
	responseChan := make(chan ping.ServerResponse)

	for _, server := range servers {
		promRespTimeout.SetValue([]string{server.Name, server.Group, promTypeConnectTimeout}, 0)
		promRespTimeout.SetValue([]string{server.Name, server.Group, promTypeServerMsgTimeout}, 0)
		promRespTimeout.SetValue([]string{server.Name, server.Group, promTypeTransferTimeout}, 0)
		promRespErr.SetValue([]string{server.Name, server.Group}, 0)
	}

	statsLogTime := time.Now()
	statistics := make(map[string]ping.Statistics)
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

			stat, statExist := statistics[resp.Name]
			if !statExist {
				stat = ping.Statistics{
					ServerName:  resp.Name,
					ServerGroup: resp.Group,
				}
			}

			if resp.ConnectDuration == 0 {
				promConnTime.Delete([]string{resp.Name, resp.Group})
			} else {
				conn := int(resp.ConnectDuration.Milliseconds())
				promConnTime.SetValue([]string{resp.Name, resp.Group}, conn)
				stat.ConnectDurations = append(stat.ConnectDurations, conn)
			}

			if resp.PingDuration == 0 {
				promRespTime.Delete([]string{resp.Name, resp.Group})
			} else {
				ping := int(resp.PingDuration.Milliseconds())
				promRespTime.SetValue([]string{resp.Name, resp.Group}, ping)
				stat.PingDurations = append(stat.PingDurations, ping)
			}

			if resp.Error != nil {
				if errors.Is(resp.Error, ping.ErrConnectTimeout) {
					promRespTimeout.AddValue([]string{resp.Name, resp.Group, promTypeConnectTimeout}, 1)
					stat.Timeouts1++
				} else if errors.Is(resp.Error, ping.ErrServerMsgTimeout) {
					promRespTimeout.AddValue([]string{resp.Name, resp.Group, promTypeServerMsgTimeout}, 1)
					stat.Timeouts2++
				} else if errors.Is(resp.Error, ping.ErrTransferTimeout) {
					promRespTimeout.AddValue([]string{resp.Name, resp.Group, promTypeTransferTimeout}, 1)
					stat.Timeouts3++
				} else {
					fmt.Printf("%v %v\n", resp.Name, resp.Error)
					promRespErr.AddValue([]string{resp.Name, resp.Group}, 1)
					stat.Errors++
				}
			}

			statistics[resp.Name] = stat
		}

		if time.Now().After(statsLogTime.Add(*STATS_INTERVAL)) {
			fmt.Printf(
				"\n%v to %v sent %v requests\n",
				statsLogTime.Format(time.TimeOnly),
				time.Now().Format(time.TimeOnly),
				requestCount,
			)
			ping.PrintResults(statistics, statsGroupOrder)
			statsLogTime = time.Now()
			statistics = make(map[string]ping.Statistics)
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

	if *STATS_COUNT != 0 {
		fmt.Printf("Stats count is %v\n", *STATS_COUNT)
	}
	fmt.Printf("Ping timeout %v\n", *PING_TIMEOUT)
	fmt.Printf("Ping interval %v\n", *PING_INTERVAL)
	fmt.Printf("Stats interval %v\n", *STATS_INTERVAL)

	var filter *regexp.Regexp
	if *FILTER != "" {
		fmt.Printf("Server name filter regexp: %v\n", *FILTER)
		filter = regexp.MustCompile(*FILTER)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	var allServers []ping.Server
	for _, configName := range configs {
		serversPath := fmt.Sprintf("./servers/%v.json", configName)
		fmt.Printf("\nServer list %v\n", serversPath)

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

		fmt.Printf("Loaded servers:\n")
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
		fmt.Println("No servers found")
		os.Exit(1)
	}

	if *LISTEN_PORT == 0 {
		fmt.Println("Listen port is not set. Prometheus metrics disabled")
		recordMetrics(allServers, configsWithComma)
	} else {
		metrics := []*ping.PrometheusMetric{
			&promRespErr,
			&promRespTime,
			&promConnTime,
			&promRespTimeout,
		}
		http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			var resp strings.Builder
			for _, metric := range metrics {
				resp.WriteString(metric.GetString())
			}
			w.Write([]byte(resp.String()))
		})

		go recordMetrics(allServers, configsWithComma)
		fmt.Printf("Listening port %v\n", *LISTEN_PORT)
		err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%v", *LISTEN_PORT), nil)
		fmt.Println(err)
	}
}
