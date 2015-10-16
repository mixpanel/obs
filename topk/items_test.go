package topk

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestItemListBasic(t *testing.T) {
	list := make(itemList, 0)

	items := make([]*item, 0)
	for idx := 0; idx < 10; idx++ {
		i := &item{int32(idx), 0, 0}
		items = append(items, i)
		list.put(i)
	}
	assert.True(t, isSorted(list))

	items[5].frequency = 10
	list.fix(items[5])
	assert.True(t, isSorted(list))

	for idx := 0; idx < 10; idx++ {
		if idx != 5 {
			items[idx].frequency = 11
			list.fix(items[idx])
		}
	}
	assert.True(t, isSorted(list))
}

func TestRandomized(t *testing.T) {
	list := make(itemList, 0)
	rng := rand.New(rand.NewSource(1232456))

	for idx := 0; idx < 100; idx++ {
		list.put(&item{int32(idx), 0, 0})
	}

	for i := 0; i < 100000; i++ {
		index := rng.Intn(len(list))
		list[index].frequency = rng.Int()
		list.fix(list[index])
	}
	assert.True(t, isSorted(list))
}

func BenchmarkRandomized(b *testing.B) {
	list := make(itemList, 0)
	rng := rand.New(rand.NewSource(1232456))

	for idx := 0; idx < 1000; idx++ {
		list.put(&item{int32(idx), 0, 0})
	}
	indexes := make([]int, 0, b.N)
	freqs := make([]int, 0, b.N)

	for i := 0; i < b.N; i++ {
		indexes = append(indexes, rng.Intn(len(list)))
		freqs = append(freqs, rng.Int())
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index := indexes[i]
		list[index].frequency = freqs[i]
		list.fix(list[index])
	}
	assert.True(b, isSorted(list))
}

func isSorted(list itemList) bool {
	ints := make([]int, 0, len(list))

	for i := len(list) - 1; i >= 0; i-- {
		ints = append(ints, list[i].frequency)
		if list[i].index != i {
			return false
		}
	}

	return sort.IntsAreSorted(ints)
}
