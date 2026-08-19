package ping

import "github.com/egoroof/wow-server-ping/pkg/wow"

type ServerConfig struct {
	// domain or ip without port
	Host string
	Port string
	// ips with port
	AddressList []string
	Realms      []wow.Realm
}

type Server struct {
	Name    string
	Address string

	Group string
}
