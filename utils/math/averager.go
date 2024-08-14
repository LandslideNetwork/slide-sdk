package math

import "time"

// Averager tracks a continuous time exponential moving average of the provided
// values.
type Averager interface {
	// Observe the value at the given time
	Observe(value float64, currentTime time.Time)

	// Read returns the average of the provided values.
	Read() float64
}
