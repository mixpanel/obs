package outlier

import (
	"hash"
	"hash/fnv"
	"obs"
	"obs/logging"
	"obs/metrics"
	"sync"
	"time"
)

type BalanceTracker interface {
	Track(key int32, value string)
	IsBalanced(key int32) bool
	Close()
}

type KeyVal struct {
	key   int32
	value string
}

type keyCount struct {
	count   int64
	buckets []uint64
}

type balanceTracker struct {
	interval              time.Duration
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
	var v *keyCount
	var ok bool
	if v, ok = t.counts[kv.key]; !ok {
		v = &keyCount{
			count:   0,
			buckets: make([]uint64, t.numBuckets),
		}
		t.counts[kv.key] = v
	}

	v.count++

	v.buckets[int(t.hash(kv.value))%len(v.buckets)]++
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
		if v.count >= t.minTrackedPerInterval {
			if !t.isArrayBalanced(v.buckets) {
				imbalanced[k] = struct{}{}
				numImbalanced++

				obs.Log.Warnf("imbalanced tracks for key", logging.Fields{
					"key":        k,
					"num_tracks": v.count,
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
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.sample()
			case kv, ok := <-t.tracks:
				if !ok {
					return
				}
				t.track(kv)
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
	interval time.Duration,
	minTrackedPerInterval int64,
	numBuckets int,
	balanceRatio float64,
	receiver metrics.Receiver,
) BalanceTracker {

	tracker := &balanceTracker{
		interval:              interval,
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
