package sampler

// Weighted defines how to sample a specified valued based on a provided
// weighted distribution
type Weighted interface {
	Initialize(weights []uint64) error
	Sample(sampleValue uint64) (int, bool)
}

// NewWeighted returns a new sampler
func NewWeighted() Weighted {
	return &weightedBest{
		samplers: []Weighted{
			&weightedArray{},
			&weightedHeap{},
			&weightedUniform{
				maxWeight: 1024,
			},
		},
		benchmarkIterations: 100,
	}
}

func NewDeterministicWeighted() Weighted {
	return &weightedHeap{}
}
