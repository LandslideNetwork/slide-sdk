package types

import (
	"encoding/json"
	"fmt"
	"github.com/cometbft/cometbft/types"
)

const (
	defaultTimeoutBroadcastTxCommit uint16 = 30 // seconds
	defaultNetworkName                     = "landslide-test"

	defaultMaxBytes int64 = 100 * 1024 * 1024 // 10MB
	defaultMaxGas   int64 = 10000000

	defaultMaxSubscriptionClients    = 100
	defaultMaxSubscriptionsPerClient = 5
)

type (
	Config struct {
		VMConfig  VMConfig        `json:"vm_config"`
		AppConfig json.RawMessage `json:"app_config"`
	}

	// VMConfig contains the configuration of the VM.
	VMConfig struct {
		NetworkName               string          `json:"network_name"`
		TimeoutBroadcastTxCommit  uint16          `json:"timeout_broadcast_tx_commit"`
		ConsensusParams           ConsensusParams `json:"consensus_params"`
		MaxSubscriptionClients    int             `json:"max_subscription_clients"`
		MaxSubscriptionsPerClient int             `json:"max_subscriptions_per_client"`
		BLSSecretKey              []byte          `json:"bls_secret_key"`
	}

	// ConsensusParams contains consensus critical parameters that determine the
	// validity of blocks.
	ConsensusParams struct {
		Block    BlockParams    `json:"block"`
		Evidence EvidenceParams `json:"evidence"`
	}
	// BlockParams contains the consensus critical parameters for a block.
	BlockParams struct {
		MaxBytes int64 `json:"max_bytes"`
		MaxGas   int64 `json:"max_gas"`
	}
	// EvidenceParams contains the consensus critical parameters for evidence.
	EvidenceParams struct {
		MaxBytes int64 `json:"max_bytes"`
	}
)

// SetDefaults sets the default values for the config.
func (c *VMConfig) SetDefaults() {
	c.NetworkName = defaultNetworkName
	c.TimeoutBroadcastTxCommit = defaultTimeoutBroadcastTxCommit

	c.ConsensusParams.Block.MaxBytes = defaultMaxBytes
	c.ConsensusParams.Block.MaxGas = defaultMaxGas
	c.ConsensusParams.Evidence.MaxBytes = 1000

	c.MaxSubscriptionsPerClient = defaultMaxSubscriptionsPerClient
	c.MaxSubscriptionClients = defaultMaxSubscriptionClients
}

// Validate returns an error if this is an invalid config.
func (c *VMConfig) Validate() error {
	if len(c.NetworkName) == 0 {
		return fmt.Errorf("network_name can't be empty")
	}

	if err := c.ConsensusParams.Validate(); err != nil {
		return fmt.Errorf("consensus_params is invalid: %w", err)
	}

	if c.MaxSubscriptionsPerClient < 0 {
		return fmt.Errorf("max_subscriptions_per_client must be positive. Got %d", c.MaxSubscriptionsPerClient)
	}

	if c.MaxSubscriptionClients < 0 {
		return fmt.Errorf("max_subscription_clients must be positive. Got %d", c.MaxSubscriptionClients)
	}

	return nil
}

// Validate returns an error if this is an invalid config.
func (c *ConsensusParams) Validate() error {
	if c.Block.MaxBytes == 0 {
		return fmt.Errorf("block.MaxBytes cannot be 0")
	}
	if c.Block.MaxBytes < -1 {
		return fmt.Errorf("block.MaxBytes must be -1 or greater than 0. Got %d", c.Block.MaxBytes)
	}
	if c.Block.MaxBytes > types.MaxBlockSizeBytes {
		return fmt.Errorf("block.MaxBytes is too big. %d > %d", c.Block.MaxBytes, types.MaxBlockSizeBytes)
	}

	if c.Block.MaxGas < -1 {
		return fmt.Errorf("block.MaxGas must be greater or equal to -1. Got %d", c.Block.MaxGas)
	}

	maxBytes := c.Block.MaxBytes
	if maxBytes == -1 {
		maxBytes = int64(types.MaxBlockSizeBytes)
	}
	if c.Evidence.MaxBytes > maxBytes {
		return fmt.Errorf("evidence.MaxBytes is greater than upper bound, %d > %d",
			c.Evidence.MaxBytes, c.Block.MaxBytes)
	}

	if c.Evidence.MaxBytes < 0 {
		return fmt.Errorf("evidence.MaxBytes must be non negative. Got: %d",
			c.Evidence.MaxBytes)
	}

	return nil
}
