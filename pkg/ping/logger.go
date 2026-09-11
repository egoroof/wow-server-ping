package ping

import (
	"errors"
	"log"
	"os"
	"sync"
)

type ErrorLogger struct {
	// key := server.Name + server.Group
	lastErrorByServer map[string]error

	logger *log.Logger

	mu sync.Mutex
}

func NewErrorLogger(filename string) *ErrorLogger {
	// todo file close at gracefull shutdown?
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}

	return &ErrorLogger{
		lastErrorByServer: make(map[string]error),
		logger:            log.New(file, "", log.LstdFlags),
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
	s.logger.Printf("%v/%v %v\n", server.Group, server.Name, res.Error)
}

func (s *ErrorLogger) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	clear(s.lastErrorByServer)
}
