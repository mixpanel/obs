package outlier

import (
	"hash/fnv"
	"obs/metrics"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newBalanceTracker(minTrackedPerInterval int64, numBuckets int) *balanceTracker {
	return &balanceTracker{
		minTrackedPerInterval: minTrackedPerInterval,
		numBuckets:            numBuckets,
		interval:              60 * time.Second,
		decayFactor:           2.0,
		balanceRatio:          0.1,
		hasher:                fnv.New32(),
		tracks:                make(chan *KeyVal, 16),
		counts:                make(map[int32]*keyCount),
		imbalanced:            make(map[int32]struct{}),
		receiver:              metrics.Null,
	}
}

func generateDistinctIds(numDistinctIds int) []string {
	ret := make([]string, numDistinctIds)

	for i := 0; i < numDistinctIds; i++ {
		ret[i] = strconv.Itoa(i)
	}

	return ret
}

func TestBalanceTracker(t *testing.T) {
	dids := generateDistinctIds(5000)
	tracker := newBalanceTracker(5000, 100)

	for i := 0; i < 5000; i++ {
		tracker.track(&KeyVal{1, dids[i]})
		if i%2 == 0 {
			tracker.track(&KeyVal{2, dids[0]})
		} else {
			tracker.track(&KeyVal{2, dids[i]})
		}
	}

	tracker.sample()

	assert.True(t, tracker.IsBalanced(1))
	assert.False(t, tracker.IsBalanced(2))
}

func TestBalanceTrackerMinUpdates(t *testing.T) {
	dids := generateDistinctIds(5000)
	tracker := newBalanceTracker(5000, 100)

	for i := 0; i < 2000; i++ {
		tracker.track(&KeyVal{1, dids[0]})
	}

	tracker.sample()
	assert.True(t, tracker.IsBalanced(1))
}

func TestBalanceTrackerDecay(t *testing.T) {
	dids := generateDistinctIds(5000)
	tracker := newBalanceTracker(5000, 100)

	for i := 0; i < 5000; i++ {
		tracker.track(&KeyVal{1, dids[0]})
	}

	tracker.sample()
	assert.False(t, tracker.IsBalanced(1))

	for i := 0; i < 100; i++ {
		for j := 0; j < 5000; j++ {
			tracker.track(&KeyVal{1, dids[j]})
		}
		tracker.sample()
	}

	assert.True(t, tracker.IsBalanced(1))
}

func TestBalanceTrackerDecayWithoutUpdates(t *testing.T) {
	dids := generateDistinctIds(1)
	tracker := newBalanceTracker(5000, 100)

	for i := 0; i < 5000; i++ {
		tracker.track(&KeyVal{1, dids[0]})
	}

	tracker.sample()
	assert.False(t, tracker.IsBalanced(1))

	for i := 0; i < 100; i++ {
		tracker.sample()
	}

	assert.True(t, tracker.IsBalanced(1))
}

func TestBalanceTrackerInterface(t *testing.T) {
	dids := generateDistinctIds(10000)
	tracker := NewBalanceTracker(1*time.Second, 5000, 100, 0.1, metrics.Null)

	for i := 0; i < 10000; i++ {
		if i%2 == 0 {
			tracker.Track(1, dids[0])
		} else {
			tracker.Track(1, dids[i])
		}
		tracker.Track(2, dids[i])
	}

	time.Sleep(1500 * time.Millisecond)
	assert.False(t, tracker.IsBalanced(1))
	assert.True(t, tracker.IsBalanced(2))

	tracker.Close()
}
