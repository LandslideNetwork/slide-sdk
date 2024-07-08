package types

import (
	"fmt"
	"time"
)

const (
	defaultRPCPort                                = 9752
	defaultGRPCPort                               = 9090
	defaultMaxOpenConnections                     = 0 // unlimited
	defaultTimeoutBroadcastTxCommit time.Duration = 10 * time.Second

	defaultNetworkName    = "landslide-test"
	defaultWarpAPIEnabled = true
)

// VMConfig ...
type VMConfig struct {
	RPCConfig      RPCConfig `json:"rpc_config"`
	NetworkName    string    `json:"network_name"`
	WarpAPIEnabled bool      `json:"warp_api_enabled"`
}

type RPCConfig struct {
	RPCPort                  uint16        `json:"rpc_port"`
	GRPCPort                 uint16        `json:"grpc_port"`
	GRPCMaxOpenConnections   int           `json:"grpc_max_open_connections"`
	TimeoutBroadcastTxCommit time.Duration `json:"broadcast_tx_commit_timeout"`
}

// SetDefaults sets the default values for the config.
func (c *VMConfig) SetDefaults() {
	c.NetworkName = defaultNetworkName
	c.WarpAPIEnabled = defaultWarpAPIEnabled

	c.RPCConfig.RPCPort = defaultRPCPort
	c.RPCConfig.GRPCPort = defaultGRPCPort
	c.RPCConfig.GRPCMaxOpenConnections = defaultMaxOpenConnections
	c.RPCConfig.TimeoutBroadcastTxCommit = defaultTimeoutBroadcastTxCommit
}

// Validate returns an error if this is an invalid config.
func (c *VMConfig) Validate() error {
	if c.RPCConfig.GRPCMaxOpenConnections < 0 {
		return fmt.Errorf("grpc_max_open_connections can't be negative")
	}

	if c.RPCConfig.TimeoutBroadcastTxCommit < 0 {
		return fmt.Errorf("broadcast_tx_commit_timeout can't be negative")
	}

	if len(c.NetworkName) == 0 {
		return fmt.Errorf("network_name can't be empty")
	}

	return nil
}
