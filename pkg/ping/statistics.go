package ping

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
)

type Statistics struct {
	Server       *Server
	RequestCount int

	PingDurations      []int
	ConnectDurations   []int
	HandshakeDurations []int

	Errors    int
	Timeouts1 int
	Timeouts2 int
	Timeouts3 int

	PingMean      int
	ConnectMean   int
	HandshakeMean int

	PingMAD      int
	ConnectMAD   int
	HandshakeMAD int
}

type Store struct {
	// key := server.Name
	stats map[string]*Statistics

	shortNames []string
	proxyNames []string

	writer *tabwriter.Writer

	mu sync.Mutex
}

func NewStatsStore() *Store {
	return &Store{
		stats:  make(map[string]*Statistics),
		writer: tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0),
	}
}

func (s *Store) Init(servers []*Server) {
	for _, server := range servers {
		s.stats[server.Name] = &Statistics{
			Server: server,
		}

		if !server.IsAuth {
			if !slices.Contains(s.shortNames, server.ShortName) {
				s.shortNames = append(s.shortNames, server.ShortName)
			}
			if !slices.Contains(s.proxyNames, server.ProxyName) {
				s.proxyNames = append(s.proxyNames, server.ProxyName)
			}
		}
	}
}

func (s *Store) Update(server *Server, res *PingResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stat := s.stats[server.Name]
	stat.RequestCount++

	if res.ConnectDuration != 0 {
		conn := int(res.ConnectDuration.Milliseconds())
		stat.ConnectDurations = append(stat.ConnectDurations, conn)
	}

	if res.HandshakeDuration != 0 {
		handshake := int(res.HandshakeDuration.Milliseconds())
		stat.HandshakeDurations = append(stat.HandshakeDurations, handshake)
	}

	if res.PingDuration != 0 {
		ping := int(res.PingDuration.Milliseconds())
		stat.PingDurations = append(stat.PingDurations, ping)
	}

	if res.Error != nil {
		if errors.Is(res.Error, ErrConnectTimeout) {
			stat.Timeouts1++
		} else if errors.Is(res.Error, ErrHandshakeTimeout) {
			stat.Timeouts2++
		} else if errors.Is(res.Error, ErrPingTimeout) {
			stat.Timeouts3++
		} else {
			stat.Errors++
		}
	}

	s.stats[server.Name] = stat
}

func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, elem := range s.stats {
		s.stats[key] = &Statistics{
			Server: elem.Server,
		}
	}
}

func (s *Store) Print() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// key1 proxyName, key2 shortName
	realmStats := make(map[string]map[string]*Statistics)
	var authStats []*Statistics
	for _, stats := range s.stats {
		stats.PingMean = Mean(stats.PingDurations)
		stats.PingMAD = MAD(stats.PingDurations)

		stats.ConnectMean = Mean(stats.ConnectDurations)
		stats.ConnectMAD = MAD(stats.ConnectDurations)

		stats.HandshakeMean = Mean(stats.HandshakeDurations)
		stats.HandshakeMAD = MAD(stats.HandshakeDurations)

		if stats.Server.IsAuth {
			authStats = append(authStats, stats)
		} else {
			if _, ok := realmStats[stats.Server.ProxyName]; !ok {
				realmStats[stats.Server.ProxyName] = make(map[string]*Statistics)
			}
			realmStats[stats.Server.ProxyName][stats.Server.ShortName] = stats
		}
	}

	slices.SortFunc(s.proxyNames, func(a, b string) int {
		proxyGroupA := realmStats[a]
		proxyGroupB := realmStats[b]

		pingSumA := 0
		pingSumB := 0
		for _, name := range s.shortNames {
			if statsA, ok := proxyGroupA[name]; ok {
				pingSumA += statsA.PingMean
			}
			if statsB, ok := proxyGroupB[name]; ok {
				pingSumB += statsB.PingMean
			}
		}
		// every server from the group doesn't have ping (cause timeouts or errors)
		// so move it down in the list
		if pingSumA == 0 {
			return 1
		}
		if pingSumB == 0 {
			return -1
		}

		pingGroupA := pingSumA / len(proxyGroupA)
		pingGroupB := pingSumB / len(proxyGroupB)

		return pingGroupA - pingGroupB
	})

	if len(authStats) > 0 {
		slices.SortFunc(authStats, func(a, b *Statistics) int {
			return strings.Compare(a.Server.ShortName, b.Server.ShortName)
		})

		for _, stats := range authStats {
			fmt.Fprintf(s.writer, "\t%v", stats.Server.ShortName)
		}
		fmt.Fprintf(s.writer, "\nMain")
		for _, stats := range authStats {
			fmt.Fprintf(s.writer, "\t%v", formatStats(stats, true))
		}
		fmt.Fprintf(s.writer, "\n\n")
	}

	for _, name := range s.shortNames {
		fmt.Fprintf(s.writer, "\t%v", name)
	}
	fmt.Fprintf(s.writer, "\n")

	for _, proxy := range s.proxyNames {
		fmt.Fprintf(s.writer, "%v", proxy)
		for _, name := range s.shortNames {
			fmt.Fprintf(s.writer, "\t%v", formatStats(realmStats[proxy][name], false))
		}
		fmt.Fprintf(s.writer, "\n")
	}

	s.writer.Flush()
}

func formatStats(stats *Statistics, useHandshake bool) string {
	if stats == nil {
		return "-"
	}

	delayStr := ""
	delayDurationsLen := len(stats.PingDurations)
	delayMean := stats.PingMean

	if useHandshake {
		delayDurationsLen = len(stats.HandshakeDurations)
		delayMean = stats.HandshakeMean
	}

	if delayDurationsLen == 0 {
		delayStr = ""
	} else if delayMean == 0 {
		delayStr = "<1"
	} else {
		delayStr = strconv.Itoa(delayMean)
	}

	madStr := ""
	mad := stats.PingMAD

	if useHandshake {
		mad = stats.HandshakeMAD
	}

	if mad > 100 {
		madStr = "***"
	} else if mad > 50 {
		madStr = "**"
	} else if mad > 10 {
		madStr = "*"
	}

	errorsStr := ""
	if stats.Timeouts1 > 0 {
		errorsStr += "(T1)"
	}
	if stats.Timeouts2 > 0 {
		errorsStr += "(T2)"
	}
	if stats.Timeouts3 > 0 {
		errorsStr += "(T3)"
	}
	if stats.Errors > 0 {
		errorsStr += "(E)"
	}

	return delayStr + madStr + errorsStr
}
