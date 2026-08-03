package prometheus

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/egoroof/wow-server-ping/pkg/ping"
)

const typeConnectTimeout = "connect"
const typeServerMsgTimeout = "serverMsg"
const typeTransferTimeout = "transfer"

type ResponseMetrics struct {
	connTime    metric
	respTime    metric
	respTimeout metric
	respError   metric
}

func NewResponseMetrics() *ResponseMetrics {
	m := ResponseMetrics{
		connTime: metric{
			name:       "wow_server_connect_time_ms",
			help:       "WoW server connect time in ms",
			typee:      "gauge",
			labelNames: []string{"server", "group"},
		},
		respTime: metric{
			name:       "wow_server_response_time_ms",
			help:       "WoW server response time in ms",
			typee:      "gauge",
			labelNames: []string{"server", "group"},
		},
		respTimeout: metric{
			name:       "wow_server_timeout_count",
			help:       "WoW server timeout count",
			typee:      "counter",
			labelNames: []string{"server", "group", "type"},
		},
		respError: metric{
			name:       "wow_server_error_count",
			help:       "WoW server error count",
			typee:      "counter",
			labelNames: []string{"server", "group"},
		},
	}
	return &m
}

func (m *ResponseMetrics) ListenAndServe(port int) error {
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		var resp strings.Builder
		resp.WriteString(m.respError.string())
		resp.WriteString(m.respTime.string())
		resp.WriteString(m.connTime.string())
		resp.WriteString(m.respTimeout.string())
		w.Write([]byte(resp.String()))
	})

	return http.ListenAndServe(fmt.Sprintf("127.0.0.1:%v", port), nil)
}

func (m *ResponseMetrics) Init(servers []*ping.Server) {
	for _, server := range servers {
		m.respTimeout.setValue([]string{server.Name, server.Group, typeConnectTimeout}, 0)
		m.respError.setValue([]string{server.Name, server.Group}, 0)

		if !server.ConnectOnly {
			m.respTimeout.setValue([]string{server.Name, server.Group, typeServerMsgTimeout}, 0)
			m.respTimeout.setValue([]string{server.Name, server.Group, typeTransferTimeout}, 0)
		}
	}
}

func (m *ResponseMetrics) Update(res *ping.PingResult) {
	if res.ConnectDuration == 0 {
		m.connTime.delete([]string{res.Name, res.Group})
	} else {
		conn := int(res.ConnectDuration.Milliseconds())
		m.connTime.setValue([]string{res.Name, res.Group}, conn)
	}

	if res.PingDuration == 0 {
		m.respTime.delete([]string{res.Name, res.Group})
	} else {
		ping := int(res.PingDuration.Milliseconds())
		m.respTime.setValue([]string{res.Name, res.Group}, ping)
	}

	if res.Error != nil {
		if errors.Is(res.Error, ping.ErrConnectTimeout) {
			m.respTimeout.addValue([]string{res.Name, res.Group, typeConnectTimeout}, 1)
		} else if errors.Is(res.Error, ping.ErrServerMsgTimeout) {
			m.respTimeout.addValue([]string{res.Name, res.Group, typeServerMsgTimeout}, 1)
		} else if errors.Is(res.Error, ping.ErrTransferTimeout) {
			m.respTimeout.addValue([]string{res.Name, res.Group, typeTransferTimeout}, 1)
		} else {
			m.respError.addValue([]string{res.Name, res.Group}, 1)
		}
	}
}
