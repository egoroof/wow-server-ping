package ping

type Server struct {
	Name        string
	Address     string
	ConnectOnly bool

	Group string
}
