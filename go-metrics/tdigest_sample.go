package metrics

import (
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stripe/veneur/tdigest"
)

const (
	bucketWidth = time.Minute
)

type bucket struct {
	earliest      time.Time
	values, count int64
	digest        *tdigest.MergingDigest
}

func (b *bucket) merge(other *bucket) {
	if b.earliest.After(other.earliest) {
		b.earliest = other.earliest
	}
	b.count += other.count
	b.values += other.values
	b.digest.Merge(other.digest)
}

func (sample *TDigestSample) newBucket() *bucket {
	return &bucket{
		earliest: sample.clock.Now(),
		// The higher the first number passed to NewMerging, the more accurate the result
		digest: tdigest.NewMerging(200.0, false),
	}
}

type TDigestSample struct {
	timeWindow time.Duration
	clock      clockwork.Clock

	mutex   sync.RWMutex
	count   int64 // Total number of updates seen by the sample
	cur     *bucket
	buckets []*bucket
}

func NewTDigestSample(timeWindow time.Duration, clock clockwork.Clock) *TDigestSample {
	sample := &TDigestSample{
		timeWindow: timeWindow,
		clock:      clock,
	}
	sample.cur = sample.newBucket()
	return sample
}

func (sample *TDigestSample) drop() {
	i := 0
	cutoff := sample.clock.Now().Add(-sample.timeWindow).Add(-time.Second)
	for ; i < len(sample.buckets); i++ {
		if sample.buckets[i].earliest.After(cutoff) {
			break
		}
	}
	sample.buckets = sample.buckets[i:]
}

func (sample *TDigestSample) Update(value int64) {
	sample.mutex.Lock()
	defer sample.mutex.Unlock()

	sample.count++
	now := sample.clock.Now()
	if now.Sub(sample.cur.earliest) >= bucketWidth {
		sample.buckets = append(sample.buckets, sample.cur)
		sample.drop()
		sample.cur = sample.newBucket()
	}

	sample.cur.values += value
	sample.cur.count++
	sample.cur.digest.Add(float64(value), 1.0)
}

func (sample *TDigestSample) Clear() {
	sample.mutex.Lock()
	defer sample.mutex.Unlock()

	sample.count = 0
	sample.buckets = nil
	sample.cur = sample.newBucket()
}

func (sample *TDigestSample) Count() int64 {
	sample.mutex.RLock()
	defer sample.mutex.RUnlock()

	return sample.count
}

func (sample *TDigestSample) Dropped() int64 {
	return 0
}

func (sample *TDigestSample) Size() int {
	panic("not implemented")
}

func (sample *TDigestSample) Values() []int64 {
	panic("not implemented")
}

func (sample *TDigestSample) merged() *bucket {
	if len(sample.buckets) == 0 {
		return sample.cur
	}

	ret := sample.newBucket()
	for _, b := range sample.buckets {
		ret.merge(b)
	}
	ret.merge(sample.cur)
	return ret
}

func (sample *TDigestSample) Max() int64 {
	sample.mutex.RLock()
	defer sample.mutex.RUnlock()

	merged := sample.merged()
	if merged.count == 0 {
		return 0
	}
	return int64(merged.digest.Max())
}

func (sample *TDigestSample) Mean() float64 {
	sample.mutex.RLock()
	defer sample.mutex.RUnlock()

	merged := sample.merged()
	if merged.count == 0 {
		return 0.0
	}
	return float64(merged.values) / float64(merged.count)
}

func (sample *TDigestSample) Min() int64 {
	sample.mutex.RLock()
	defer sample.mutex.RUnlock()

	merged := sample.merged()
	if merged.count == 0 {
		return 0
	}
	return int64(merged.digest.Min())
}

func (sample *TDigestSample) Percentile(percentile float64) float64 {
	sample.mutex.RLock()
	defer sample.mutex.RUnlock()

	merged := sample.merged()
	if merged.count == 0 {
		return 0.0
	}
	return merged.digest.Quantile(percentile)
}

func (sample *TDigestSample) Percentiles(percentiles []float64) []float64 {
	sample.mutex.RLock()
	defer sample.mutex.RUnlock()

	merged := sample.merged()
	ret := make([]float64, 0, len(percentiles))
	for _, p := range percentiles {
		ret = append(ret, merged.digest.Quantile(p))
	}
	return ret
}

func (sample *TDigestSample) Snapshot() Sample {
	sample.mutex.RLock()
	defer sample.mutex.RUnlock()

	sample.drop()
	merged := sample.merged()
	cp := *merged.digest
	return &TDigestSample{
		timeWindow: sample.timeWindow,
		count:      sample.count,
		cur: &bucket{
			earliest: merged.earliest,
			count:    merged.count,
			values:   merged.values,
			digest:   &cp,
		},
	}
}

func (sample *TDigestSample) StdDev() float64 {
	panic("not implemented")
}

func (sample *TDigestSample) Sum() int64 {
	sample.mutex.RLock()
	defer sample.mutex.RUnlock()

	return sample.merged().values
}

func (sample *TDigestSample) Variance() float64 {
	panic("not implemented")
}
