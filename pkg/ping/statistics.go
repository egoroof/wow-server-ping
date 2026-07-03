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

	PingAvg    int
	ConnectAvg int

	PingJitter    int
	ConnectJitter int
}

func PrintResults(statistics map[string]Statistics, groupsOrder string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	groups := strings.Split(groupsOrder, ",")

	serverTableGroups := make(map[string][]Statistics)
	for _, stats := range statistics {
		stats.PingAvg = Avg(stats.PingDurations)
		stats.PingJitter = Jitter(stats.PingDurations)
		stats.ConnectAvg = Avg(stats.ConnectDurations)
		stats.ConnectJitter = Jitter(stats.ConnectDurations)

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
			return a.PingAvg - b.PingAvg
		})
	}

	fmt.Fprintf(w, "Realm\tConn\t±\tPing\t±\tT1\tT2\tT3\tE\n")
	for _, group := range groups {
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
			if stats.PingAvg == 0 {
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
				stats.ConnectAvg, stats.ConnectJitter, stats.PingAvg, stats.PingJitter,
				t1, t2, t3, e,
			)
		}
		w.Flush()
		if len(serverTableGroups) > 1 {
			fmt.Println("")
		}
	}
}
