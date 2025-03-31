// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package aggregator

import (
	"context"
	"fmt"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/warp"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/landslidenetwork/slide-sdk/utils/crypto/bls"
	"github.com/landslidenetwork/slide-sdk/utils/set"
)

const WarpQuorumDenominator uint64 = 100

type AggregateSignatureResult struct {
	// Weight of validators included in the aggregate signature.
	SignatureWeight uint64
	// Total weight of all validators in the subnets.
	TotalWeight uint64
	// The message with the aggregate signature.
	Message *warp.Message
}

type signatureFetchResult struct {
	sig    *bls.Signature
	index  int
	weight uint64
}

// Aggregator requests signatures from validators and
// aggregates them into a single signature.
type Aggregator struct {
	logger      log.Logger
	validators  []*warp.Validator
	totalWeight uint64
	client      SignatureGetter
}

// New returns a signature aggregator that will attempt to aggregate signatures from [validators].
func New(client SignatureGetter, logger log.Logger, validators []*warp.Validator, totalWeight uint64) *Aggregator {
	return &Aggregator{
		client:      client,
		logger:      logger,
		validators:  validators,
		totalWeight: totalWeight,
	}
}

// Returns an aggregate signature over [unsignedMessage].
// The returned signature's weight exceeds the threshold given by [quorumNum].
func (a *Aggregator) AggregateSignatures(ctx context.Context, unsignedMessage *warp.UnsignedMessage, quorumNum uint64) (*AggregateSignatureResult, error) {
	// Create a child context to cancel signature fetching if we reach signature threshold.
	signatureFetchCtx, signatureFetchCancel := context.WithCancel(ctx)
	defer signatureFetchCancel()

	// Fetch signatures from validators concurrently.
	signatureFetchResultChan := make(chan *signatureFetchResult)
	for i, validator := range a.validators {
		var (
			i         = i
			validator = validator
			// TODO: update from a single nodeID to the original slice and use extra nodeIDs as backup.
			nodeID = validator.NodeIDs[0]
		)
		go func() {
			a.logger.Debug("Fetching warp signature",
				"nodeID", nodeID,
				"index", i,
				"msgID", unsignedMessage.ID(),
			)

			signature, err := a.client.GetSignature(signatureFetchCtx, nodeID, unsignedMessage)
			if err != nil {
				a.logger.Debug("Failed to fetch warp signature",
					"nodeID", nodeID,
					"index", i,
					"err", err,
					"msgID", unsignedMessage.ID(),
				)
				signatureFetchResultChan <- nil
				return
			}

			a.logger.Debug("Retrieved warp signature",
				"nodeID", nodeID,
				"msgID", unsignedMessage.ID(),
				"index", i,
			)

			if !bls.Verify(validator.PublicKey, signature, unsignedMessage.Bytes()) {
				a.logger.Debug("Failed to verify warp signature",
					"nodeID", nodeID,
					"index", i,
					"msgID", unsignedMessage.ID(),
					//TODO: remove!! WARNING,
					"signature", signature,
					"pubKey", validator.PublicKey,
				)
				signatureFetchResultChan <- nil
				return
			}

			signatureFetchResultChan <- &signatureFetchResult{
				sig:    signature,
				index:  i,
				weight: validator.Weight,
			}
		}()
	}

	var (
		signatures                = make([]*bls.Signature, 0, len(a.validators))
		signersBitset             = set.NewBits()
		signaturesWeight          = uint64(0)
		signaturesPassedThreshold = false
	)
	a.logger.Info("START signature fetching")

	a.logger.Info("amount of validators", len(a.validators))
	for i := 0; i < len(a.validators); i++ {
		signatureFetchResult := <-signatureFetchResultChan
		if signatureFetchResult == nil {
			a.logger.Info("WARNING: nil result of signature fetch process")
			continue
		}

		signatures = append(signatures, signatureFetchResult.sig)
		signersBitset.Add(signatureFetchResult.index)
		signaturesWeight += signatureFetchResult.weight
		a.logger.Debug("Updated weight",
			"totalWeight", signaturesWeight,
			"addedWeight", signatureFetchResult.weight,
			"msgID", unsignedMessage.ID(),
		)

		// If the signature weight meets the requested threshold, cancel signature fetching
		if err := warp.VerifyWeight(signaturesWeight, a.totalWeight, quorumNum, WarpQuorumDenominator); err == nil {
			a.logger.Debug("Verify weight passed, exiting aggregation early",
				"quorumNum", quorumNum,
				"totalWeight", a.totalWeight,
				"signatureWeight", signaturesWeight,
				"msgID", unsignedMessage.ID(),
			)
			signatureFetchCancel()
			signaturesPassedThreshold = true
			break
		} else {
			a.logger.Error("ERROR WARP AGGREGATE SIGNATURE VERIFY WEIGHT: INSUFFICIENT WEIGHT, weight", signaturesWeight, "totalWeight", a.totalWeight, "quorumNum", quorumNum, "quorumDenominator", WarpQuorumDenominator)
		}
	}

	// If I failed to fetch sufficient signature stake, return an error
	if !signaturesPassedThreshold {
		a.logger.Error("ERROR WARP AGGREGATE SIGNATURE: INSUFFICIENT WEIGHT")
		return nil, warp.ErrInsufficientWeight
	}

	// Otherwise, return the aggregate signature
	aggregateSignature, err := bls.AggregateSignatures(signatures)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate BLS signatures: %w", err)
	}

	warpSignature := &warp.BitSetSignature{
		Signers: signersBitset.Bytes(),
	}
	copy(warpSignature.Signature[:], bls.SignatureToBytes(aggregateSignature))

	msg, err := warp.NewMessage(unsignedMessage, warpSignature)
	if err != nil {
		return nil, fmt.Errorf("failed to construct warp message: %w", err)
	}

	return &AggregateSignatureResult{
		Message:         msg,
		SignatureWeight: signaturesWeight,
		TotalWeight:     a.totalWeight,
	}, nil
}
