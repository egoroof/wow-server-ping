package resolver

import (
	"context"
	"net"
	"time"
)

var resolver = &net.Resolver{}

func LookupHost(host string, timeout time.Duration) (ips []string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return resolver.LookupHost(ctx, host)
}
