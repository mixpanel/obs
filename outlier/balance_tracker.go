package outlier

import (
	"fmt"
	"hash"
	"hash/fnv"
	"obs"
	"obs/logging"
	"obs/metrics"
	"sync"
)

type BalanceTracker interface {
	Track(key int32, value string)
	IsBalanced(key int32) bool
	Close()
}

type NullBalanceTracker struct{}

func (t *NullBalanceTracker) Track(key int32, value string) {}
func (t *NullBalanceTracker) Close()                        {}
func (t *NullBalanceTracker) IsBalanced(key int32) bool     { return true }

type KeyVal struct {
	key   int32
	value string
}

type keyCount struct {
	count   int64
	buckets []uint64

	shouldLog      bool
	distinctValues map[string]int64
}

type balanceTracker struct {
	sampleEvery           int
	decayFactor           float64
	balanceRatio          float64
	minTrackedPerInterval int64
	numBuckets            int

	tracks   chan *KeyVal
	counts   map[int32]*keyCount
	hasher   hash.Hash32
	receiver metrics.Receiver

	mutex      sync.RWMutex
	imbalanced map[int32]struct{}
}

func (t *balanceTracker) hash(value string) uint32 {
	defer t.hasher.Reset()
	t.hasher.Write([]byte(value))
	return t.hasher.Sum32()
}

func (t *balanceTracker) track(kv *KeyVal) {
	if len(kv.value) == 0 {
		return
	}
	var v *keyCount
	var ok bool
	if v, ok = t.counts[kv.key]; !ok {
		v = &keyCount{
			count:          0,
			buckets:        make([]uint64, t.numBuckets),
			shouldLog:      false,
			distinctValues: make(map[string]int64),
		}
		t.counts[kv.key] = v
	}

	v.count++

	v.buckets[int(t.hash(kv.value))%len(v.buckets)]++

	if v.shouldLog {
		v.distinctValues[kv.value]++
	}
}

func (t *balanceTracker) isArrayBalanced(values []uint64) bool {
	largest := uint64(0)
	sum := uint64(0)
	for _, v := range values {
		sum += v
		if largest < v {
			largest = v
		}
	}

	if largest > uint64(t.balanceRatio*float64(sum)) {
		return false
	}

	return true
}

func (t *balanceTracker) sample() {
	imbalanced := make(map[int32]struct{})
	counts := make(map[int32]*keyCount)

	numImbalanced := 0
	for k, v := range t.counts {

		if v.shouldLog {
			m := make(map[string]int64)
			for d, c := range v.distinctValues {
				if c > int64(float64(v.count)*0.05) {
					m[d] = c
				}
			}
			// Note that these values are captured one cycle after the associated key is marked as unbalanced
			// As a result, the numbers here might not be the same as those that caused the key to become
			// unbalanced in the first place.
			obs.Log.Warn("distinct values with count larger than 5 percent of the total", logging.Fields{
				"key":             k,
				"count":           v.count,
				"distinct_values": m,
			})
		}

		v.shouldLog = false
		v.distinctValues = make(map[string]int64)

		if v.count >= t.minTrackedPerInterval {
			if !t.isArrayBalanced(v.buckets) {
				v.shouldLog = true
				imbalanced[k] = struct{}{}
				numImbalanced++

				t.receiver.Incr(fmt.Sprintf("imbalanced.%d", k))
				obs.Log.Warn("imbalanced tracks for key", logging.Fields{
					"key":        k,
					"num_tracks": v.count,
					"buckets":    v.buckets,
				})
			}
		}

		v.count = 0
		shouldKeep := false

		for idx, value := range v.buckets {
			v.buckets[idx] = uint64(float64(value) / t.decayFactor)
			shouldKeep = (shouldKeep || v.buckets[idx] > 0)
		}

		if shouldKeep {
			counts[k] = v
		}
	}

	// NOTE: We add a stat here instead of a gauge because our implementation of statsd
	// 		 reports values for gauges even if they aren't updated.
	t.receiver.AddStat("num_imbalanced_keys", float64(numImbalanced))

	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.imbalanced = imbalanced
	t.counts = counts
}

func (t *balanceTracker) start() {
	go func() {
		count := 0
		for {
			kv, ok := <-t.tracks
			if !ok {
				return
			}
			t.track(kv)

			count++
			if count >= t.sampleEvery {
				count = 0
				t.sample()
			}
		}
	}()
}

func (t *balanceTracker) IsBalanced(key int32) bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	_, ok := t.imbalanced[key]
	return !ok
}

func (t *balanceTracker) Track(key int32, value string) {
	select {
	case t.tracks <- &KeyVal{key, value}:
	default:
	}
}

func (t *balanceTracker) Close() {
	close(t.tracks)
}

func NewBalanceTracker(
	sampleEvery int,
	minTrackedPerInterval int64,
	numBuckets int,
	balanceRatio float64,
	receiver metrics.Receiver,
) BalanceTracker {

	tracker := &balanceTracker{
		sampleEvery:           sampleEvery,
		balanceRatio:          balanceRatio,
		minTrackedPerInterval: minTrackedPerInterval,
		numBuckets:            numBuckets,
		receiver:              receiver,
		decayFactor:           2.0,
		hasher:                fnv.New32(),
		tracks:                make(chan *KeyVal, 8192),
		counts:                make(map[int32]*keyCount),
		imbalanced:            make(map[int32]struct{}),
	}

	tracker.start()

	return tracker
}
