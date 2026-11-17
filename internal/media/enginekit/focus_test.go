// Adapted from jplayer-mobile/internal/media/enginekit/focus_test.go at 49de3c2.

package enginekit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFocus(t *testing.T) {
	t.Parallel()

	type step struct {
		change  int32 // 0: a request granted
		news    FocusNews
		wanted  bool // Wanted after the step
		holding bool // Held after the step
	}
	tests := []struct {
		name  string
		steps []step
	}{
		{"the first play requests", []step{
			{0, FocusNews{}, false, true},
		}},
		{"a transient loss and its gain resume", []step{
			{0, FocusNews{}, false, true},
			{FocusLossTransient, FocusNews{Begin: true}, true, true},
			{FocusGain, FocusNews{End: true, Resume: true}, false, true},
		}},
		{"a play during a transient loss requests again, and the gain after resumes nothing", []step{
			{0, FocusNews{}, false, true},
			{FocusLossTransient, FocusNews{Begin: true}, true, true},
			{0, FocusNews{}, false, true},
			{FocusGain, FocusNews{}, false, true},
		}},
		{"a loss for good ends at once and does not resume", []step{
			{0, FocusNews{}, false, true},
			{FocusLoss, FocusNews{Begin: true, End: true}, true, false},
			{FocusGain, FocusNews{}, false, true},
		}},
		{"a loss for good after a transient one", []step{
			{0, FocusNews{}, false, true},
			{FocusLossTransient, FocusNews{Begin: true}, true, true},
			{FocusLoss, FocusNews{Begin: true, End: true}, true, false},
		}},
		{"ducking is the system's", []step{
			{0, FocusNews{}, false, true},
			{FocusLossDuck, FocusNews{}, false, true},
			{FocusGain, FocusNews{}, false, true},
		}},
		{"an unknown change is nothing", []step{
			{0, FocusNews{}, false, true},
			{42, FocusNews{}, false, true},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var f Focus
			assert.True(t, f.Wanted(), "not held at first")
			assert.False(t, f.Held())
			for i, s := range tt.steps {
				var news FocusNews
				if s.change == 0 {
					f.Granted()
				} else {
					news = f.Change(s.change)
				}
				assert.Equal(t, s.news, news, "step %d", i)
				assert.Equal(t, s.wanted, f.Wanted(), "step %d: Wanted", i)
				assert.Equal(t, s.holding, f.Held(), "step %d: Held", i)
			}
			f.Abandoned()
			assert.True(t, f.Wanted())
			assert.False(t, f.Held())
		})
	}
}
