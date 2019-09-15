package metrics

import (
	"sync"
	"time"
)

type timedValue struct {
	timestamp int64
	value     int64
}

type TimeWindowSample struct {
	startWindowSize int
	maxWindowSize   int
	timeWindow      time.Duration
	scaleFactor     float64

	mutex sync.Mutex // guards everything below

	count     int64 // Total number of updates seen by the sample
	dropped   int64 // Total number of dropped points that are in the time window
	values    []timedValue
	numValues int // number of active values being tracked by this sample
	latest    int
	earliest  int
}

func NewTimeWindowSample(startWindowSize int, maxWindowSize int, timeWindow time.Duration) Sample {
	return &TimeWindowSample{
		startWindowSize: startWindowSize,
		maxWindowSize:   maxWindowSize,
		timeWindow:      timeWindow,
		scaleFactor:     1.5,
		values:          make([]timedValue, startWindowSize),
	}
}

func (sample *TimeWindowSample) resize(size int) {
	if size < sample.numValues {
		panic("size must be greater than sample numValues")
	}

	previous := sample.values
	sample.values = make([]timedValue, size)

	copy(sample.values, previous)

	sample.earliest = 0
	sample.latest = int(sample.numValues - 1)
}

func (sample *TimeWindowSample) drop(forced int, nowNano int64) {
	numValues := sample.numValues
	permitAfterNano := nowNano - sample.timeWindow.Nanoseconds()

	advance := func() {
		forced--
		sample.numValues--

		if sample.values[sample.earliest].timestamp > permitAfterNano {
			sample.dropped++
		}

		sample.earliest = (sample.earliest + 1) % len(sample.values)
	}

	for i := 0; i < numValues; i++ {
		if forced > 0 || sample.values[sample.earliest].timestamp <= permitAfterNano {
			advance()
		} else {
			break
		}
	}
}

func (sample *TimeWindowSample) add(value int64, nowNano int64) {
	if sample.numValues >= len(sample.values) {
		panic("cannot add to sample that is already full")
	}

	if sample.numValues == 0 {
		sample.earliest, sample.latest = 0, 0
	} else {
		sample.latest = (sample.latest + 1) % len(sample.values)
	}

	sample.numValues++
	sample.values[sample.latest].value = value
	sample.values[sample.latest].timestamp = nowNano
}

func (sample *TimeWindowSample) Update(value int64) {
	sample.mutex.Lock()
	defer sample.mutex.Unlock()

	sample.count++
	now := time.Now().UnixNano()

	// scale up if needed
	if sample.numValues == len(sample.values) {
		if sample.numValues < sample.maxWindowSize {
			newSize := int(float64(len(sample.values)) * sample.scaleFactor)
			if newSize > sample.maxWindowSize {
				newSize = sample.maxWindowSize
			}
			sample.resize(newSize)
		} else {
			sample.drop(1, now)
		}
	}

	sample.add(value, now)

	// scale down if needed
	if int(float64(3*sample.numValues)*sample.scaleFactor) < len(sample.values) {
		newSize := int(float64(len(sample.values)) / sample.scaleFactor)
		if newSize < sample.startWindowSize {
			newSize = sample.startWindowSize
		}
		sample.resize(newSize)
	}
}

func (sample *TimeWindowSample) Clear() {
	sample.mutex.Lock()
	defer sample.mutex.Unlock()

	sample.numValues = 0
	sample.count = 0
}

func (sample *TimeWindowSample) Count() int64 {
	sample.mutex.Lock()
	defer sample.mutex.Unlock()

	return sample.count
}

func (sample *TimeWindowSample) Dropped() int64 {
	sample.mutex.Lock()
	defer sample.mutex.Unlock()

	return sample.dropped
}

func (sample *TimeWindowSample) Size() int {
	sample.mutex.Lock()
	defer sample.mutex.Unlock()

	return sample.numValues
}

func (sample *TimeWindowSample) Values() []int64 {
	sample.mutex.Lock()
	defer sample.mutex.Unlock()

	sample.drop(0, time.Now().UnixNano())

	values := make([]int64, sample.numValues)
	for i, idx := 0, sample.earliest; i < sample.numValues; i, idx = i+1, (idx+1)%len(sample.values) {
		values[i] = sample.values[idx].value
	}

	return values
}

func (sample *TimeWindowSample) Max() int64 {
	return SampleMax(sample.Values())
}

func (sample *TimeWindowSample) Mean() float64 {
	return SampleMean(sample.Values())
}

func (sample *TimeWindowSample) Min() int64 {
	return SampleMin(sample.Values())
}

func (sample *TimeWindowSample) Percentile(percentile float64) float64 {
	return SamplePercentile(sample.Values(), percentile)
}

func (sample *TimeWindowSample) Percentiles(percentiles []float64) []float64 {
	return SamplePercentiles(sample.Values(), percentiles)
}

func (sample *TimeWindowSample) Snapshot() Sample {
	values := sample.Values()
	return &SampleSnapshot{
		count:   sample.count,
		dropped: sample.dropped,
		values:  values,
	}
}

func (sample *TimeWindowSample) StdDev() float64 {
	return SampleStdDev(sample.Values())
}

func (sample *TimeWindowSample) Sum() int64 {
	return SampleSum(sample.Values())
}

func (sample *TimeWindowSample) Variance() float64 {
	return SampleVariance(sample.Values())
}
