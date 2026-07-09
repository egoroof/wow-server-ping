package ping

import (
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

func PrintResults(statistics map[string]Statistics, groupsOrder string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	groups := strings.Split(groupsOrder, ",")

	serverTableGroups := make(map[string][]Statistics)
	for _, stats := range statistics {
		stats.PingMean = Mean(stats.PingDurations)
		stats.PingMAD = MAD(stats.PingDurations)

		stats.ConnectMean = Mean(stats.ConnectDurations)
		stats.ConnectMAD = MAD(stats.ConnectDurations)

		serverTableGroups[stats.ServerGroup] = append(serverTableGroups[stats.ServerGroup], stats)
	}
	for group := range serverTableGroups {
		slices.SortFunc(serverTableGroups[group], func(a, b Statistics) int {
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
			return a.PingMAD - b.PingMAD
		})
	}

	for _, group := range groups {
		fmt.Fprintf(w, "Realm\tConn\t±\tPing\t±\tT1\tT2\tT3\tE\n")
		for _, stats := range serverTableGroups[group] {
			t1 := " "
			t2 := " "
			t3 := " "
			e := " "
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
			if stats.PingMean == 0 {
				fmt.Fprintf(
					w, "%v\tunavailable\t\t\t\t%v\t%v\t%v\t%v\n",
					stats.ServerName,
					t1, t2, t3, e,
				)
				continue
			}
			fmt.Fprintf(
				w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
				stats.ServerName,
				stats.ConnectMean, stats.ConnectMAD,
				stats.PingMean, stats.PingMAD,
				t1, t2, t3, e,
			)
		}
		if len(serverTableGroups) > 1 {
			fmt.Fprintf(w, "\n")
		}
	}
	w.Flush()
}
