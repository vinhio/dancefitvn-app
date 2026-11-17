package avftv

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPictureDue(t *testing.T) {
	t.Parallel()

	shown := pictureReading{video: true, attached: true, ready: true, w: 480, h: 854}
	with := func(f func(r *pictureReading)) pictureReading {
		r := shown
		f(&r)

		return r
	}
	cases := []struct {
		name string
		r    pictureReading
		w, h int
		due  bool
	}{
		{"first frame shown", shown, 480, 854, true},
		{"size rounded", with(func(r *pictureReading) { r.w, r.h = 853.33, 480 }), 853, 480, true},
		{"told once a load", with(func(r *pictureReading) { r.told = true }), 0, 0, false},
		{"audio only", with(func(r *pictureReading) { r.video = false }), 0, 0, false},
		{"failed load", with(func(r *pictureReading) { r.failed = true }), 0, 0, false},
		{"layer without the player", with(func(r *pictureReading) { r.attached = false }), 0, 0, false},
		{"layer not ready", with(func(r *pictureReading) { r.ready = false }), 0, 0, false},
		{"size unknown", with(func(r *pictureReading) { r.w, r.h = 0, 0 }), 0, 0, false},
		{"height unknown", with(func(r *pictureReading) { r.h = 0 }), 0, 0, false},
		{"size NaN", with(func(r *pictureReading) { r.w = math.NaN() }), 0, 0, false},
		{"size infinite", with(func(r *pictureReading) { r.h = math.Inf(1) }), 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			w, h, due := pictureDue(c.r)
			assert.Equal(t, c.due, due)
			assert.Equal(t, c.w, w)
			assert.Equal(t, c.h, h)
		})
	}
}

func TestPictureDueDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	r := pictureReading{video: true, attached: true, ready: true, w: 480, h: 854}
	assert.Zero(t, testing.AllocsPerRun(100, func() { _, _, _ = pictureDue(r) }))
}

func BenchmarkPictureDue(b *testing.B) {
	r := pictureReading{video: true, attached: true, ready: true, w: 480, h: 854}
	b.ReportAllocs()
	for b.Loop() {
		_, _, _ = pictureDue(r)
	}
}
