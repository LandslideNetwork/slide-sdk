// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package timeout

import (
	"time"

	"github.com/landslidenetwork/slide-sdk/utils/avalanche/networking/benchlist"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/utils/math"
	"github.com/prometheus/client_golang/prometheus"
)

// Manages timeouts for requests sent to peers.
type Manager interface {
	// TimeoutDuration returns the current timeout duration.
	TimeoutDuration() time.Duration
	// IsBenched returns true if messages to [nodeID] regarding [chainID]
	// should not be sent over the network and should immediately fail.
	IsBenched(nodeID ids.NodeID) bool
	// Registers that we would have sent a request to a validator but they
	// are unreachable because they are benched or because of network conditions
	// (e.g. we're not connected), so we didn't send the query. For the sake
	// of calculating the average latency and network timeout, we act as
	// though we sent the validator a request and it timed out.
	RegisterRequestToUnreachableValidator()
}

func NewManager(
	nwBenchlist benchlist.Benchlist,
) (Manager, error) {
	return &manager{
		benchlist: nwBenchlist,
	}, nil
}

type manager struct {
	// Averages the response time from all peers
	averager       math.Averager
	currentTimeout time.Duration // Amount of time before a timeout
	minimumTimeout time.Duration
	maximumTimeout time.Duration
	// Timeout is [timeoutCoefficient] * average response time
	// [timeoutCoefficient] must be > 1
	timeoutCoefficient               float64
	networkTimeoutMetric, avgLatency prometheus.Gauge
	benchlist                        benchlist.Benchlist
}

func (m *manager) TimeoutDuration() time.Duration {
	return m.currentTimeout
}

// IsBenched returns true if messages to [nodeID]
// should not be sent over the network and should immediately fail.
func (m *manager) IsBenched(nodeID ids.NodeID) bool {
	return m.benchlist.IsBenched(nodeID)
}

func (m *manager) RegisterRequestToUnreachableValidator() {
	latency := m.TimeoutDuration()
	now := time.Now()
	m.averager.Observe(float64(latency), now)
	avgLatency := m.averager.Read()
	m.currentTimeout = time.Duration(m.timeoutCoefficient * avgLatency)
	if m.currentTimeout > m.maximumTimeout {
		m.currentTimeout = m.maximumTimeout
	} else if m.currentTimeout < m.minimumTimeout {
		m.currentTimeout = m.minimumTimeout
	}
	// Update the metrics
	m.networkTimeoutMetric.Set(float64(m.currentTimeout))
	m.avgLatency.Set(avgLatency)
}
