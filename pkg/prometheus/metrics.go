package prometheus

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/egoroof/wow-server-ping/pkg/ping"
)

const typeConnectTimeout = "connect"
const typeHandshakeTimeout = "handshake"
const typePingTimeout = "ping"

type ResponseMetrics struct {
	connectTime   metric
	handshakeTime metric
	pingTime      metric
	timeouts      metric
	errors        metric
}

func NewResponseMetrics() *ResponseMetrics {
	return &ResponseMetrics{
		connectTime: metric{
			name:       "wow_server_connect_time_ms",
			help:       "WoW server connect time in ms",
			typee:      "gauge",
			labelNames: []string{"server", "group"},
		},
		handshakeTime: metric{
			name:       "wow_server_handshake_time_ms",
			help:       "WoW server handshake time in ms",
			typee:      "gauge",
			labelNames: []string{"server", "group"},
		},
		pingTime: metric{
			name:       "wow_server_ping_time_ms",
			help:       "WoW server ping time in ms",
			typee:      "gauge",
			labelNames: []string{"server", "group"},
		},
		timeouts: metric{
			name:       "wow_server_timeout_count",
			help:       "WoW server timeout count",
			typee:      "counter",
			labelNames: []string{"server", "group", "type"},
		},
		errors: metric{
			name:       "wow_server_error_count",
			help:       "WoW server error count",
			typee:      "counter",
			labelNames: []string{"server", "group"},
		},
	}
}

func (m *ResponseMetrics) ListenAndServe(port int) error {
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		var resp strings.Builder
		resp.WriteString(m.errors.string())
		resp.WriteString(m.pingTime.string())
		resp.WriteString(m.connectTime.string())
		resp.WriteString(m.handshakeTime.string())
		resp.WriteString(m.timeouts.string())
		w.Write([]byte(resp.String()))
	})

	return http.ListenAndServe(fmt.Sprintf("127.0.0.1:%v", port), nil)
}

func (m *ResponseMetrics) Init(servers []*ping.Server) {
	for _, server := range servers {
		m.timeouts.setValue([]string{server.Name, server.Group, typeConnectTimeout}, 0)
		m.errors.setValue([]string{server.Name, server.Group}, 0)

		if !server.ConnectOnly {
			m.timeouts.setValue([]string{server.Name, server.Group, typeHandshakeTimeout}, 0)
			m.timeouts.setValue([]string{server.Name, server.Group, typePingTimeout}, 0)
		}
	}
}

func (m *ResponseMetrics) Update(res *ping.PingResult) {
	if res.ConnectDuration == 0 {
		m.connectTime.delete([]string{res.Name, res.Group})
	} else {
		conn := int(res.ConnectDuration.Milliseconds())
		m.connectTime.setValue([]string{res.Name, res.Group}, conn)
	}

	if res.HandshakeDuration == 0 {
		m.handshakeTime.delete([]string{res.Name, res.Group})
	} else {
		hand := int(res.HandshakeDuration.Milliseconds())
		m.handshakeTime.setValue([]string{res.Name, res.Group}, hand)
	}

	if res.PingDuration == 0 {
		m.pingTime.delete([]string{res.Name, res.Group})
	} else {
		ping := int(res.PingDuration.Milliseconds())
		m.pingTime.setValue([]string{res.Name, res.Group}, ping)
	}

	if res.Error != nil {
		if errors.Is(res.Error, ping.ErrConnectTimeout) {
			m.timeouts.addValue([]string{res.Name, res.Group, typeConnectTimeout}, 1)
		} else if errors.Is(res.Error, ping.ErrHandshakeTimeout) {
			m.timeouts.addValue([]string{res.Name, res.Group, typeHandshakeTimeout}, 1)
		} else if errors.Is(res.Error, ping.ErrTransferTimeout) {
			m.timeouts.addValue([]string{res.Name, res.Group, typePingTimeout}, 1)
		} else {
			m.errors.addValue([]string{res.Name, res.Group}, 1)
		}
	}
}
