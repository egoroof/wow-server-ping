package prometheus

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

type metric struct {
	name       string
	help       string
	typee      string // gauge | counter
	labelNames []string
	elems      []metricElem

	mu sync.Mutex
}

func (m *metric) setValue(labels []string, value int) {
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

func (m *metric) addValue(labels []string, value int) {
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

func (m *metric) delete(labels []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, elem := range m.elems {
		if slices.Equal(elem.labels, labels) {
			m.elems = slices.Delete(m.elems, i, i+1)
			return
		}
	}
}

func (m *metric) string() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var res strings.Builder
	fmt.Fprintf(&res, "# HELP %v %v\n", m.name, m.help)
	fmt.Fprintf(&res, "# TYPE %v %v\n", m.name, m.typee)

	for _, elem := range m.elems {
		fmt.Fprintf(&res, "%v{", m.name)

		for i, labelName := range m.labelNames {
			fmt.Fprintf(&res, `%v="%v"`, labelName, elem.labels[i])
			if i != len(m.labelNames)-1 {
				fmt.Fprintf(&res, " ")
			}
		}

		fmt.Fprintf(&res, "} %v\n", elem.value)
	}

	fmt.Fprintf(&res, "\n")
	return res.String()
}
