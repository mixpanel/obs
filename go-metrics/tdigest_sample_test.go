package metrics

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
)

const (
	prime = 15485867
)

func equalWithinBound(bound, expected, received float64) bool {
	if expected == 0.0 {
		return math.Abs(expected-received) < 1e-6
	}
	diff := math.Abs(float64(expected-received) / float64(expected))
	return bound > diff
}

func assertEqualWithinBound(t *testing.T, bound, expected, received float64) {
	assert.True(t, equalWithinBound(bound, expected, received))
}

func TestTDigestSample(t *testing.T) {
	table := map[string]int64{
		"no values":   0,
		"100 values":  100,
		"100K values": 100000,
		"1M values":   1000000,
	}

	for name, n := range table {
		t.Run(name, func(t *testing.T) {

			sample := NewTDigestSample(5*time.Minute, clockwork.NewFakeClock())
			var i int64
			var samples []int64
			for c := int64(0); c < n; c++ {
				sample.Update(i + 1)
				samples = append(samples, i+1)
				i = (i + prime) % n
			}

			snap := sample.Snapshot()
			assert.Equal(t, n, snap.Count())
			assert.Equal(t, SampleSum(samples), snap.Sum())
			assert.Equal(t, SampleMin(samples), snap.Min())
			assert.Equal(t, SampleMax(samples), snap.Max())
			assertEqualWithinBound(t, 0.0001, SampleMean(samples), snap.Mean())

			for _, p := range []float64{0.5, 0.9, 0.99} {
				assertEqualWithinBound(t, 0.05, SamplePercentile(samples, p), snap.Percentile(p))
			}
		})
	}
}

func TestTDigestSampleTimeWindow(t *testing.T) {
	n := int64(1000)
	type fixture struct {
		ranges          []int64
		expectedBuckets []int
	}

	table := map[string]*fixture{
		"3 minutes": &fixture{
			ranges:          []int64{0, 200, 500, 1000},
			expectedBuckets: []int{0, 1, 2},
		},
		"4 minutes": &fixture{
			ranges:          []int64{0, 20, 200, 500, 1000},
			expectedBuckets: []int{0, 1, 2, 3},
		},
		"5 minutes": &fixture{
			ranges:          []int64{0, 20, 200, 500, 700, 1000},
			expectedBuckets: []int{0, 1, 2, 3, 4},
		},
		"6 minutes": &fixture{
			ranges:          []int64{0, 20, 300, 500, 700, 800, 1000},
			expectedBuckets: []int{0, 1, 2, 3, 4, 5},
		},
		"7 minutes": &fixture{
			ranges:          []int64{0, 20, 300, 500, 700, 800, 900, 1000},
			expectedBuckets: []int{0, 1, 2, 3, 4, 5, 5},
		},
	}

	for name, row := range table {
		t.Run(name, func(t *testing.T) {
			clock := clockwork.NewFakeClock()
			sample := NewTDigestSample(5*time.Minute, clock)

			var samples []int64
			for i := 1; i < len(row.ranges); i++ {
				for j := row.ranges[i-1]; j < row.ranges[i]; j++ {
					sample.Update(j + 1)
					if len(row.ranges)-i < 6 {
						samples = append(samples, j+1)
					}
				}
				assert.Equal(t, row.expectedBuckets[i-1], len(sample.buckets))
				clock.Advance(bucketWidth)
			}

			snap := sample.Snapshot()
			assert.Equal(t, n, snap.Count())
			assert.Equal(t, SampleSum(samples), snap.Sum())
			assert.Equal(t, SampleMin(samples), snap.Min())
			assert.Equal(t, SampleMax(samples), snap.Max())
			assertEqualWithinBound(t, 0.0001, SampleMean(samples), snap.Mean())

			for _, p := range []float64{0.5, 0.9, 0.99} {
				assertEqualWithinBound(t, 0.05, SamplePercentile(samples, p), snap.Percentile(p))
			}
		})
	}
}

func BenchmarkTDigestSample(b *testing.B) {
	rand.Seed(31)
	sample := NewTDigestSample(5*time.Minute, clockwork.NewRealClock())
	for i := 0; i < b.N; i++ {
		sample.Update(rand.Int63())
	}
}

func BenchmarkTDigestSamplePercentile(b *testing.B) {
	rand.Seed(31)
	sample := NewTDigestSample(5*time.Minute, clockwork.NewRealClock())

	var samples []int64
	for i := 0; i < 100000; i++ {
		v := rand.Int31()
		sample.Update(int64(v))
		samples = append(samples, int64(v))
	}

	pVals := []float64{0.5, 0.9, 0.99}
	percentiles := map[float64]float64{
		0.5:  SamplePercentile(samples, 0.5),
		0.9:  SamplePercentile(samples, 0.9),
		0.99: SamplePercentile(samples, 0.99),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := pVals[i%len(pVals)]
		v := sample.Percentile(p)
		if !equalWithinBound(0.05, percentiles[p], v) {
			panic(fmt.Sprintf("values differ by more than 5 percent: expected: %v, received: %v", percentiles[p], v))
		}
	}
}
