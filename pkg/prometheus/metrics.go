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

func (m *ResponseMetrics) Update(server *ping.Server, res *ping.PingResult) {
	promKey := []string{server.Name, server.Group}
	if res.ConnectDuration == 0 {
		m.connectTime.delete(promKey)
	} else {
		conn := int(res.ConnectDuration.Milliseconds())
		m.connectTime.setValue(promKey, conn)
	}

	if res.HandshakeDuration == 0 {
		m.handshakeTime.delete(promKey)
	} else {
		hand := int(res.HandshakeDuration.Milliseconds())
		m.handshakeTime.setValue(promKey, hand)
	}

	if res.PingDuration == 0 {
		m.pingTime.delete(promKey)
	} else {
		ping := int(res.PingDuration.Milliseconds())
		m.pingTime.setValue(promKey, ping)
	}

	if res.Error != nil {
		if errors.Is(res.Error, ping.ErrConnectTimeout) {
			key := append(promKey, typeConnectTimeout)
			m.timeouts.addValue(key, 1)
		} else if errors.Is(res.Error, ping.ErrHandshakeTimeout) {
			key := append(promKey, typeHandshakeTimeout)
			m.timeouts.addValue(key, 1)
		} else if errors.Is(res.Error, ping.ErrPingTimeout) {
			key := append(promKey, typePingTimeout)
			m.timeouts.addValue(key, 1)
		} else {
			m.errors.addValue(promKey, 1)
		}
	}
}
