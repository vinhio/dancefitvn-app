package store_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"dancefitvn-app/internal/store"
)

func TestOnline(t *testing.T) {
	t.Parallel()

	o := store.NewOnline()
	assert.True(t, o.Online(), "online until told otherwise")
	var got []bool
	o.Subscribe(func(on bool) {
		got = append(got, on)
		assert.Equal(t, on, o.Online(), "subscribers may call back")
	})
	o.Set(true) // no change
	o.Set(false)
	o.Set(false)
	o.Set(true)
	assert.Equal(t, []bool{false, true}, got, "only changes are told")
}

func TestOnlineConcurrent(t *testing.T) {
	t.Parallel()

	o := store.NewOnline()
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			o.Set(i%2 == 0)
			_ = o.Online()
		})
	}
	wg.Wait()
}
