package ping

type Realm struct {
	Name      string
	Address   string
	ShortName string
	ProxyName string
}

type ServerConfig struct {
	// auth server domains or ips optionally with port
	RealmList []string

	// game servers
	Realms []Realm
}

type Server struct {
	Name       string
	Address    string
	ShortName  string
	ProxyName  string
	ConfigName string
	IsAuth     bool
}
