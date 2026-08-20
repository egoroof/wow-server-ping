package ping

import "github.com/egoroof/wow-server-ping/pkg/wow"

type ServerConfig struct {
	// domain or ip without port
	Host    string
	Port    string
	HostIps []string
	Realms  []wow.Realm
}

type Server struct {
	Name    string
	Address string
	IsAuth  bool

	Group string
}
