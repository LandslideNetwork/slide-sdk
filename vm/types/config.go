package types

import (
	"fmt"
	"time"
)

const (
	defaultRPCPort                                = 9752
	defaultGRPCPort                               = 9090
	defaultMaxOpenConnections                     = 0 // unlimited
	defaultTimeoutBroadcastTxCommit time.Duration = 30 * time.Second
)

// VMConfig ...
type VMConfig struct {
	RPCPort                  uint16        `json:"rpc_port"`
	GRPCPort                 uint16        `json:"grpc_port"`
	GRPCMaxOpenConnections   int           `json:"grpc_max_open_connections"`
	TimeoutBroadcastTxCommit time.Duration `json:"broadcast_commit_timeout"`
	NetworkName              string        `json:"network_name"`
}

// SetDefaults sets the default values for the config.
func (c *VMConfig) SetDefaults() {
	c.RPCPort = defaultRPCPort
	c.GRPCPort = defaultGRPCPort
	c.GRPCMaxOpenConnections = defaultMaxOpenConnections
	c.TimeoutBroadcastTxCommit = defaultTimeoutBroadcastTxCommit
	c.NetworkName = "landslide-test"
}

// Validate returns an error if this is an invalid config.
func (c *VMConfig) Validate() error {
	if c.GRPCMaxOpenConnections < 0 {
		return fmt.Errorf("grpc_max_open_connections can't be negative")
	}

	if c.TimeoutBroadcastTxCommit < 0 {
		return fmt.Errorf("broadcast_tx_commit_timeout can't be negative")
	}

	if len(c.NetworkName) == 0 {
		return fmt.Errorf("network_name can't be empty")
	}

	return nil
}
