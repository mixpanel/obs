package topk

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopKBasic(t *testing.T) {
	tracker := newTracker(1)

	frequentValue := int32(12345)

	for i := int32(0); i < int32(bufferSize); i++ {
		tracker.track(i)
		if i%3 == 0 {
			tracker.track(frequentValue)
		}
	}

	assert.True(t, tracker.isTopK(frequentValue))

	for i := int32(0); i < 100; i++ {
		assert.False(t, tracker.isTopK(i))
	}
}

func TestTopKRandomized(t *testing.T) {
	tracker := newTracker(30)

	rng := rand.New(rand.NewSource(1223456))
	count := 10000
	items := make([]int32, 0, count)

	freq := 1
	seen := make(map[int32]struct{})
	for len(items) < count {
		value := rng.Int31()
		if _, ok := seen[value]; !ok {
			seen[value] = struct{}{}
			for i := 0; i < freq; i++ {
				items = append(items, value)
			}
			freq++
		}
	}

	shuffle(items)

	for _, value := range items {
		tracker.track(value)
	}

	toEval := items[count-8192 : count]
	top, bottom := manualCount(toEval, 30)

	for _, v := range top {
		assert.True(t, tracker.isTopK(v))
	}

	for _, v := range bottom {
		assert.False(t, tracker.isTopK(v))
	}

}

func TestTopKWithVeryFrequentItem(t *testing.T) {
	tracker := newTracker(20)
	realSeed := rand.Int63()
	defer rand.Seed(realSeed)
	rand.Seed(12345)

	for i := int32(0); i < 200000; i++ {
		tracker.track(i % 4)
	}
	for i := 0; i < 12; i++ {
		tracker.track(10)
		tracker.track(11)
		tracker.track(12)
	}
	for i := int32(0); i < int32(bufferSize); i++ {
		tracker.track(1 + (i % 4))
	}

	assert.True(t, tracker.isTopK(1))
	assert.True(t, tracker.isTopK(2))
	assert.True(t, tracker.isTopK(3))
	assert.True(t, tracker.isTopK(4))
	assert.True(t, tracker.isTopK(10))
	assert.True(t, tracker.isTopK(11))
	assert.True(t, tracker.isTopK(12))

}

func manualCount(list []int32, k int) (top []int32, bottom []int32) {
	top = make([]int32, 0, k)
	bottom = make([]int32, 0, len(list)-k)

	frequency := make(map[int32]int)

	for _, v := range list {
		frequency[v]++
	}

	sorted := make(sortableItems, 0, k)
	for key, freq := range frequency {
		sorted = append(sorted, sortableItem{key, freq})
	}

	sort.Sort(sorted)

	for i := 0; i < len(sorted); i++ {
		if i < k {
			top = append(top, sorted[i].value)
		} else {
			bottom = append(bottom, sorted[i].value)
		}
	}

	return
}

type sortableItem struct {
	value     int32
	frequency int
}

type sortableItems []sortableItem

func (l sortableItems) Len() int {
	return len(l)
}

func (l sortableItems) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l sortableItems) Less(i, j int) bool {
	return l[i].frequency > l[j].frequency
}

func shuffle(a []int32) {
	for i := range a {
		j := rand.Intn(i + 1)
		a[i], a[j] = a[j], a[i]
	}
}
