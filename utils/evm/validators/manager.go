// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validators

import (
	"context"
	"fmt"
	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	validators "github.com/landslidenetwork/slide-sdk/utils/evm/validators/state"
	"sync"
	"time"

	"github.com/landslidenetwork/slide-sdk/utils/avalanche/timer/mockable"
	avalancheuptime "github.com/landslidenetwork/slide-sdk/utils/avalanche/uptime"
	avalanchevalidators "github.com/landslidenetwork/slide-sdk/utils/avalanche/validators"
	"github.com/landslidenetwork/slide-sdk/utils/evm/validators/interfaces"
	stateinterfaces "github.com/landslidenetwork/slide-sdk/utils/evm/validators/state/interfaces"
	"github.com/landslidenetwork/slide-sdk/utils/evm/validators/uptime"
	uptimeinterfaces "github.com/landslidenetwork/slide-sdk/utils/evm/validators/uptime/interfaces"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

const (
	SyncFrequency = 1 * time.Minute
)

type manager struct {
	log            log.Logger
	Lock           sync.RWMutex
	ValidatorState avalanchevalidators.State
	SubnetID       ids.ID
	stateinterfaces.State
	uptimeinterfaces.PausableManager
}

// NewManager returns a new validator manager
// that manages the validator state and the uptime manager.
func NewManager(
	db dbm.DB,
	clock *mockable.Clock,
) (interfaces.Manager, error) {
	validatorState, err := validators.NewState(db)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize validator state: %w", err)
	}

	// Initialize uptime manager
	uptimeManager := uptime.NewPausableManager(avalancheuptime.NewManager(validatorState, clock))
	validatorState.RegisterListener(uptimeManager)

	return &manager{
		State:           validatorState,
		PausableManager: uptimeManager,
	}, nil
}

// GetValidatorAndUptime returns the calculated uptime of the validator specified by validationID
// and the last updated time.
// GetValidatorAndUptime holds the chain context lock while performing the operation and can be called concurrently.
func (m *manager) GetValidatorAndUptime(validationID ids.ID) (stateinterfaces.Validator, time.Duration, time.Time, error) {
	// lock the state
	m.Lock.RLock()
	defer m.Lock.RUnlock()

	// Get validator first
	vdr, err := m.GetValidator(validationID)
	if err != nil {
		return stateinterfaces.Validator{}, 0, time.Time{}, fmt.Errorf("failed to get validator: %w", err)
	}

	uptime, lastUpdated, err := m.CalculateUptime(vdr.NodeID)
	if err != nil {
		return stateinterfaces.Validator{}, 0, time.Time{}, fmt.Errorf("failed to get uptime: %w", err)
	}

	return vdr, uptime, lastUpdated, nil
}

// DispatchSync starts the sync process
// DispatchSync holds the chain context lock while performing the sync.
func (m *manager) DispatchSync(ctx context.Context) {
	ticker := time.NewTicker(SyncFrequency)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.Lock.Lock()
			if err := m.Sync(ctx); err != nil {
				m.log.Error("failed to sync validators", "error", err)
			}
			m.Lock.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

// Sync synchronizes the validator state with the current validator set
// and writes the state to the database.
// Sync is not safe to call concurrently and should be called with the chain context locked.
func (m *manager) Sync(ctx context.Context) error {
	now := time.Now()
	m.log.Debug("performing validator sync")
	// get current validator set
	currentValidatorSet, _, err := m.ValidatorState.GetCurrentValidatorSet(ctx, m.SubnetID)
	if err != nil {
		return fmt.Errorf("failed to get current validator set: %w", err)
	}

	// load the current validator set into the validator state
	if err := loadValidators(m.State, currentValidatorSet); err != nil {
		return fmt.Errorf("failed to load current validators: %w", err)
	}

	// write validators to the database
	if err := m.State.WriteState(); err != nil {
		return fmt.Errorf("failed to write validator state: %w", err)
	}

	// TODO: add metrics
	m.log.Debug("validator sync complete", "duration", time.Since(now))
	return nil
}

// loadValidators loads the [validators] into the validator state [validatorState]
func loadValidators(validatorState stateinterfaces.State, newValidators map[ids.ID]*avalanchevalidators.GetCurrentValidatorOutput) error {
	currentValidationIDs := validatorState.GetValidationIDs()
	// first check if we need to delete any existing validators
	for vID := range currentValidationIDs {
		// if the validator is not in the new set of validators
		// delete the validator
		if _, exists := newValidators[vID]; !exists {
			validatorState.DeleteValidator(vID)
		}
	}

	// then load the new validators
	for newVID, newVdr := range newValidators {
		currentVdr := stateinterfaces.Validator{
			ValidationID:   newVID,
			NodeID:         newVdr.NodeID,
			Weight:         newVdr.Weight,
			StartTimestamp: newVdr.StartTime,
			IsActive:       newVdr.IsActive,
			IsL1Validator:  newVdr.IsL1Validator,
		}
		if currentValidationIDs.Contains(newVID) {
			if err := validatorState.UpdateValidator(currentVdr); err != nil {
				return err
			}
		} else {
			if err := validatorState.AddValidator(currentVdr); err != nil {
				return err
			}
		}
	}
	return nil
}
