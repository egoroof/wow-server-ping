package ping

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
)

type Statistics struct {
	ServerName       string
	ServerGroup      string
	PingDurations    []int
	ConnectDurations []int
	Errors           int
	Timeouts1        int
	Timeouts2        int
	Timeouts3        int

	PingMean    int
	ConnectMean int

	PingMAD    int
	ConnectMAD int
}

type Store struct {
	// key := server.Name + server.Group
	stats map[string]*Statistics

	groups []string

	writer *tabwriter.Writer
}

func NewStatsStore(groupsOrder string) *Store {
	return &Store{
		stats:  make(map[string]*Statistics),
		groups: strings.Split(groupsOrder, ","),
		writer: tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0),
	}
}

func (s *Store) Init(servers []*Server) {
	for _, server := range servers {
		key := server.Name + server.Group
		s.stats[key] = &Statistics{
			ServerName:  server.Name,
			ServerGroup: server.Group,
		}
	}
}

func (s *Store) Update(res *PingResult) {
	key := res.Name + res.Group
	stat := s.stats[key]

	if res.ConnectDuration != 0 {
		conn := int(res.ConnectDuration.Milliseconds())
		stat.ConnectDurations = append(stat.ConnectDurations, conn)
	}

	if res.PingDuration != 0 {
		ping := int(res.PingDuration.Milliseconds())
		stat.PingDurations = append(stat.PingDurations, ping)
	}

	if res.Error != nil {
		if errors.Is(res.Error, ErrConnectTimeout) {
			stat.Timeouts1++
		} else if errors.Is(res.Error, ErrServerMsgTimeout) {
			stat.Timeouts2++
		} else if errors.Is(res.Error, ErrTransferTimeout) {
			stat.Timeouts3++
		} else {
			stat.Errors++
		}
	}

	s.stats[key] = stat
}

func (s *Store) Reset() {
	for key, elem := range s.stats {
		s.stats[key] = &Statistics{
			ServerName:  elem.ServerName,
			ServerGroup: elem.ServerGroup,
		}
	}
}

func (s *Store) Print() {
	serverTableGroups := make(map[string][]*Statistics)
	for _, stats := range s.stats {
		stats.PingMean = Mean(stats.PingDurations)
		stats.PingMAD = MAD(stats.PingDurations)

		stats.ConnectMean = Mean(stats.ConnectDurations)
		stats.ConnectMAD = MAD(stats.ConnectDurations)

		serverTableGroups[stats.ServerGroup] = append(serverTableGroups[stats.ServerGroup], stats)
	}
	for group := range serverTableGroups {
		slices.SortFunc(serverTableGroups[group], func(a, b *Statistics) int {
			if a.Errors-b.Errors != 0 {
				return a.Errors - b.Errors
			}
			aTimeouts := a.Timeouts1 + a.Timeouts2 + a.Timeouts3
			bTimeouts := b.Timeouts1 + b.Timeouts2 + b.Timeouts3
			if aTimeouts-bTimeouts != 0 {
				return aTimeouts - bTimeouts
			}
			if a.PingMean-b.PingMean != 0 {
				return a.PingMean - b.PingMean
			}
			if a.PingMAD-b.PingMAD != 0 {
				return a.PingMAD - b.PingMAD
			}
			if a.ConnectMean-b.ConnectMean != 0 {
				return a.ConnectMean - b.ConnectMean
			}
			return a.ConnectMAD - b.ConnectMAD
		})
	}

	for _, group := range s.groups {
		if _, exist := serverTableGroups[group]; !exist {
			// groups can be with zero realms due to filtering
			continue
		}
		fmt.Fprintf(s.writer, "Realm\tConn\t±\tPing\t±\tT1\tT2\tT3\tE\n")
		for _, stats := range serverTableGroups[group] {
			t1 := ""
			t2 := ""
			t3 := ""
			e := ""
			if stats.Timeouts1 > 0 {
				t1 = strconv.Itoa(stats.Timeouts1)
			}
			if stats.Timeouts2 > 0 {
				t2 = strconv.Itoa(stats.Timeouts2)
			}
			if stats.Timeouts3 > 0 {
				t3 = strconv.Itoa(stats.Timeouts3)
			}
			if stats.Errors > 0 {
				e = strconv.Itoa(stats.Errors)
			}

			connMean := strconv.Itoa(stats.ConnectMean)
			connMad := strconv.Itoa(stats.ConnectMAD)

			if len(stats.ConnectDurations) == 0 {
				connMean = "-"
				connMad = ""
			} else if stats.ConnectMean == 0 {
				connMean = "<1"
			}

			pingMean := strconv.Itoa(stats.PingMean)
			pingMad := strconv.Itoa(stats.PingMAD)

			if len(stats.PingDurations) == 0 {
				pingMean = "-"
				pingMad = ""
			} else if stats.PingMean == 0 {
				pingMean = "<1"
			}

			fmt.Fprintf(
				s.writer, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
				stats.ServerName,
				connMean, connMad,
				pingMean, pingMad,
				t1, t2, t3, e,
			)
		}
		if len(serverTableGroups) > 1 {
			fmt.Fprintf(s.writer, "\n")
		}
	}
	s.writer.Flush()
}
