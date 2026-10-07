package ping

type Realm struct {
	Name      string
	Address   string
	ShortName string
	ProxyName string
}

type ServerConfig struct {
	// domain or ip without port
	Host    string
	Port    string
	HostIps []string
	Realms  []Realm
}

type Server struct {
	Name       string
	Address    string
	ShortName  string
	ProxyName  string
	ConfigName string
	IsAuth     bool
}
