package types

import (
	"fmt"
	"time"
)

const (
	defaultGRPCPort                               = 9090
	defaultMaxOpenConnections                     = 0 // unlimited
	defaultTimeoutBroadcastTxCommit time.Duration = 30 * time.Second
)

// VmConfig ...
type VmConfig struct {
	GRPCPort                 uint16        `json:"grpc_port"`
	GRPCMaxOpenConnections   int           `json:"grpc_max_open_connections"`
	TimeoutBroadcastTxCommit time.Duration `json:"broadcast_commit_timeout"`
}

// SetDefaults sets the default values for the config.
func (c *VmConfig) SetDefaults() {
	c.GRPCPort = defaultGRPCPort
	c.GRPCMaxOpenConnections = defaultMaxOpenConnections
	c.TimeoutBroadcastTxCommit = defaultTimeoutBroadcastTxCommit
}

// Validate returns an error if this is an invalid config.
func (c *VmConfig) Validate() error {
	if c.GRPCMaxOpenConnections < 0 {
		return fmt.Errorf("grpc_max_open_connections can't be negative")
	}

	if c.TimeoutBroadcastTxCommit < 0 {
		return fmt.Errorf("broadcast_tx_commit_timeout can't be negative")
	}

	return nil
}
