// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bloom

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/landslidenetwork/slide-sdk/utils/units"
)

func TestNewErrors(t *testing.T) {
	tests := []struct {
		numHashes  int
		numEntries int
		err        error
	}{
		{
			numHashes:  0,
			numEntries: 1,
			err:        errTooFewHashes,
		},
		{
			numHashes:  17,
			numEntries: 1,
			err:        errTooManyHashes,
		},
		{
			numHashes:  8,
			numEntries: 0,
			err:        errTooFewEntries,
		},
	}
	for _, test := range tests {
		t.Run(test.err.Error(), func(t *testing.T) {
			_, err := New(test.numHashes, test.numEntries)
			require.ErrorIs(t, err, test.err)
		})
	}
}

func BenchmarkAdd(b *testing.B) {
	f, err := New(8, 16*units.KiB)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Add(1)
	}
}
