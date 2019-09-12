package outlier

import (
	"strconv"
	"testing"
	"time"

	"github.com/mixpanel/obs/metrics"

	"github.com/stretchr/testify/assert"
)

func testBalanceTracker(minTrackedPerInterval int64, numBuckets int) *balanceTracker {
	return NewBalanceTracker(1, minTrackedPerInterval, numBuckets, 0.1, metrics.Null).(*balanceTracker)
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
	tracker := testBalanceTracker(5000, 100)

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
	tracker := testBalanceTracker(5000, 100)

	for i := 0; i < 2000; i++ {
		tracker.track(&KeyVal{1, dids[0]})
	}

	tracker.sample()
	assert.True(t, tracker.IsBalanced(1))
}

func TestBalanceTrackerDecay(t *testing.T) {
	dids := generateDistinctIds(5000)
	tracker := testBalanceTracker(5000, 100)

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
	tracker := testBalanceTracker(5000, 100)

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
	dids := generateDistinctIds(1000)
	tracker := NewBalanceTracker(2000, 500, 100, 0.1, metrics.Null)
	defer tracker.Close()

	for i, did := range dids {
		if i%2 == 0 {
			tracker.Track(1, dids[0])
		} else {
			tracker.Track(1, did)
		}
		tracker.Track(2, did)
	}

	timeoutAt := time.Now().Add(10 * time.Second)
	for time.Now().Before(timeoutAt) {
		if !tracker.IsBalanced(1) && tracker.IsBalanced(2) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.FailNow()

}
