// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
	"math"
	"testing"
)

func TestGetCanonicalValidatorSet(t *testing.T) {
	//TODO: implement test
}

func TestFilterValidators(t *testing.T) {
	//TODO: implement test
}

func TestSumWeight(t *testing.T) {
	vdr0 := &Validator{
		Weight: 1,
	}
	vdr1 := &Validator{
		Weight: 2,
	}
	vdr2 := &Validator{
		Weight: math.MaxUint64,
	}

	type test struct {
		name        string
		vdrs        []*Validator
		expectedSum uint64
		expectedErr error
	}

	tests := []test{
		{
			name:        "empty",
			vdrs:        []*Validator{},
			expectedSum: 0,
		},
		{
			name:        "one",
			vdrs:        []*Validator{vdr0},
			expectedSum: 1,
		},
		{
			name:        "two",
			vdrs:        []*Validator{vdr0, vdr1},
			expectedSum: 3,
		},
		{
			name:        "overflow",
			vdrs:        []*Validator{vdr0, vdr2},
			expectedErr: ErrWeightOverflow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//TODO: implement test
		})
	}
}

func BenchmarkGetCanonicalValidatorSet(b *testing.B) {
	//TODO: implement test
}
