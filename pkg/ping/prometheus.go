package ping

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

type metricElem struct {
	labels []string
	value  int
}

type PrometheusMetric struct {
	Name       string
	Help       string
	Type       string // gauge | counter
	LabelNames []string
	elems      []metricElem

	mu sync.Mutex
}

func (m *PrometheusMetric) SetValue(labels []string, value int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, elem := range m.elems {
		if slices.Equal(elem.labels, labels) {
			m.elems[i].value = value
			return
		}
	}

	m.elems = append(m.elems, metricElem{
		labels: labels,
		value:  value,
	})
}

func (m *PrometheusMetric) AddValue(labels []string, value int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, elem := range m.elems {
		if slices.Equal(elem.labels, labels) {
			m.elems[i].value = elem.value + value
			return
		}
	}

	m.elems = append(m.elems, metricElem{
		labels: labels,
		value:  value,
	})
}

func (m *PrometheusMetric) Delete(labels []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, elem := range m.elems {
		if slices.Equal(elem.labels, labels) {
			m.elems = slices.Delete(m.elems, i, i+1)
			return
		}
	}
}

func (m *PrometheusMetric) GetString() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var res strings.Builder
	fmt.Fprintf(&res, "# HELP %v %v\n", m.Name, m.Help)
	fmt.Fprintf(&res, "# TYPE %v %v\n", m.Name, m.Type)

	for _, elem := range m.elems {
		fmt.Fprintf(&res, "%v{", m.Name)

		for i, labelName := range m.LabelNames {
			fmt.Fprintf(&res, `%v="%v"`, labelName, elem.labels[i])
			if i != len(m.LabelNames)-1 {
				fmt.Fprintf(&res, " ")
			}
		}

		fmt.Fprintf(&res, "} %v\n", elem.value)
	}

	fmt.Fprintf(&res, "\n")
	return res.String()
}
