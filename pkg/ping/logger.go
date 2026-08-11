package ping

import (
	"errors"
	"fmt"
	"sync"
)

type ErrorLogger struct {
	// key := server.Name + server.Group
	lastErrorByServer map[string]error

	mu sync.Mutex
}

func NewErrorLogger() *ErrorLogger {
	return &ErrorLogger{
		lastErrorByServer: make(map[string]error),
	}
}

// log only single error of same type for each server
func (s *ErrorLogger) Log(server *Server, res *PingResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if res.Error == nil ||
		errors.Is(res.Error, ErrConnectTimeout) ||
		errors.Is(res.Error, ErrHandshakeTimeout) ||
		errors.Is(res.Error, ErrPingTimeout) {
		return
	}

	key := server.Name + server.Group
	lastError := s.lastErrorByServer[key]

	if lastError != nil && lastError.Error() == res.Error.Error() {
		return
	}

	s.lastErrorByServer[key] = res.Error
	fmt.Printf("%v %v\n", server.Name, res.Error)
}

func (s *ErrorLogger) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	clear(s.lastErrorByServer)
}
