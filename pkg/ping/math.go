package ping

import (
	"math"
)

func Mean(values []int) int {
	if len(values) == 0 {
		return 0
	}

	sum := 0
	for _, elem := range values {
		sum += elem
	}

	return sum / len(values)
}

// Mean absolute deviation (not madness)
func MAD(values []int) int {
	mean := Mean(values)

	sum := 0.0
	for _, elem := range values {
		sum += math.Abs(float64(elem - mean))
	}

	return int(sum) / len(values)
}
