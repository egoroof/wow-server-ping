package ping

import (
	"errors"
	"fmt"
)

type ErrorLogger struct {
	// key := server.Name + server.Group
	lastErrorByServer map[string]error
}

func NewErrorLogger() *ErrorLogger {
	return &ErrorLogger{
		lastErrorByServer: make(map[string]error),
	}
}

// log only single error of same type for each server
func (s *ErrorLogger) Log(res *PingResult) {
	if res.Error == nil ||
		errors.Is(res.Error, ErrConnectTimeout) ||
		errors.Is(res.Error, ErrHandshakeTimeout) ||
		errors.Is(res.Error, ErrTransferTimeout) {
		return
	}

	key := res.Name + res.Group
	lastError := s.lastErrorByServer[key]

	if lastError != nil && lastError.Error() == res.Error.Error() {
		return
	}

	s.lastErrorByServer[key] = res.Error
	fmt.Printf("%v %v\n", res.Name, res.Error)
}

func (s *ErrorLogger) Reset() {
	clear(s.lastErrorByServer)
}
