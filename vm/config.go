package vm

import "fmt"

const (
	defaultGRPCPort           = 9090
	defaultMaxOpenConnections = 0 // unlimited
)

// Config ...
type vmConfig struct {
	GRPCPort               uint16 `json:"grpc_port"`
	GRPCMaxOpenConnections int    `json:"grpc_max_open_connections"`
}

// SetDefaults sets the default values for the config.
func (c *vmConfig) SetDefaults() {
	c.GRPCPort = defaultGRPCPort
	c.GRPCMaxOpenConnections = defaultMaxOpenConnections
}

// Validate returns an error if this is an invalid config.
func (c *vmConfig) Validate() error {
	if c.GRPCMaxOpenConnections < 0 {
		return fmt.Errorf("grpc_max_open_connections can't be negative")
	}

	return nil
}
