package resolver

import (
	"context"
	"net"
	"slices"
	"strings"
	"time"
)

var resolver = &net.Resolver{}

func LookupHost(host string, timeout time.Duration) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ips, err := resolver.LookupHost(ctx, host)

	slices.SortFunc(ips, func(a, b string) int {
		return strings.Compare(a, b)
	})

	return ips, err
}
