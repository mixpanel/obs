package topk

import "math/rand"

const (
	minSampleRate        = 0.01
	sampleRateIncrements = 0.1
	bufferSize           = 8192
)

// tracker is a ring-buffer of values, and contains a sorted list of items by frequency.
// the sorted list allows us to answer whether a particular value is in the topK.
// lastly, the tracker will adjust its sampling in attempt to capture k unique values.
type tracker struct {
	buffer     []*item
	filled     bool
	index      int
	items      map[int32]*item
	sorted     itemList
	k          int
	sampleRate float64
}

func newTracker(k int) *tracker {
	return &tracker{
		buffer:     make([]*item, bufferSize),
		filled:     false,
		index:      0,
		items:      make(map[int32]*item, k),
		sorted:     make(itemList, 0, bufferSize),
		k:          k,
		sampleRate: 1.0,
	}
}

func (t *tracker) track(value int32) bool {
	if t.filled {
		if t.sampleRate < 1.0 && rand.Float64() > t.sampleRate {
			return false
		}
		oldItem := t.buffer[t.index]
		if oldItem.frequency == 1 {
			t.sorted.remove(oldItem)
			delete(t.items, oldItem.value)
		} else {
			oldItem.frequency--
			t.sorted.fix(oldItem)
		}
	}

	if listItem, ok := t.items[value]; !ok {
		listItem = &item{value, 1, 0}
		t.items[value] = listItem
		t.sorted.put(listItem)
		t.buffer[t.index] = listItem
	} else {
		listItem.frequency++
		t.sorted.fix(listItem)
		t.buffer[t.index] = listItem
	}

	t.index = (t.index + 1) % len(t.buffer)

	if t.index == 0 {
		t.filled = true
		if t.sampleRate > minSampleRate && len(t.items) < t.k {
			newSampleRate := t.sampleRate * (1 - sampleRateIncrements)
			if newSampleRate >= minSampleRate {
				t.sampleRate = newSampleRate
			}
		} else if t.sampleRate < 1.0 && len(t.items) > t.k*2 {
			newSampleRate := t.sampleRate / (1 - sampleRateIncrements)
			if t.sampleRate <= 1.0 {
				t.sampleRate = newSampleRate
			}
		}
	}
	return true
}

func (t tracker) isTopK(value int32) bool {
	if item, ok := t.items[value]; t.filled && ok {
		return item.index < t.k
	}
	return false
}
