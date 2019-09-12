package obserr

import (
	"errors"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorsNew(t *testing.T) {
	err := errors.New("lit")
	e := &Error{
		orig: err,
		err:  err,
		vals: make(map[string]interface{}),
	}

	assert.Equal(t, e, New("lit"))
	assert.Equal(t, e, New(errors.New("lit")))
	assert.Equal(t, e, New(e))
	assert.Equal(t, "1", New(1).Error())
	assert.Equal(t, "<nil>", New(nil).Error())
}

func TestErrorsVals(t *testing.T) {
	e := New("oh?")

	assert.Equal(t, nil, e.Get("foo"))

	e.Set("key", 2)
	assert.Equal(t, 2, e.Get("key"))
	e.Set("key", 3)
	assert.Equal(t, 3, e.Get("key"))

	e.Set("a", 9, "b", 8, "c", 7)
	assert.Equal(t, 9, e.Get("a"))
	assert.Equal(t, 8, e.Get("b"))
	assert.Equal(t, 7, e.Get("c"))
	assert.Panics(t, func() {
		e.Set("z", 0, "y")
	})
	assert.Equal(t, 0, e.Get("z"))
	assert.Equal(t, e.vals, e.Vals())
}

func TestErrorsAnnotate(t *testing.T) {
	e := New("that").Annotate("see")
	assert.Equal(t, "see: that", e.Error())
	e.Annotate(errors.New("love to"))
	assert.Equal(t, "love to: see: that", e.Error())
	e.Annotate(New("you literally"))
	assert.Equal(t, "you literally: love to: see: that", e.Error())

	e = Annotate(errors.New("actually."), "but")
	assert.Equal(t, "but: actually.", e.Error())
}

func TestErrorsOriginal(t *testing.T) {
	o := errors.New("Test!")

	e := New(o).Annotate("foo").Set("a", "b").Annotate("bar")

	assert.Equal(t, o, Original(e))
	assert.Equal(t, o, Original(o))
}

func TestErrorConcurrent(t *testing.T) {
	e := New("foo")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.Set("set_by_groutine_"+strconv.Itoa(i), i)
		}()
	}

	wg.Wait()
	assert.Len(t, e.Vals(), 10)
}

func TestErrorDuplicated(t *testing.T) {
	orig := New("original")
	const concurrency = 100
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_ = Annotate(orig, "foo")
			other := Annotate(orig, "bar")
			require.Equal(t, orig.Error(), "original")
			require.Equal(t, other.Error(), Annotate(errors.New("original"), "bar").Error())
		}()
	}
	wg.Wait()
}
