//go:build tvos

// Adapted from jplayer-tv/internal/ui/focus/focus_test.go at 781bee5.

package focus_test

import (
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"

	"dancefitvn-app/internal/ui/tv/focus"
)

func TestButtonOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		key  keys.Key
		want focus.Button
		dir  focus.Dir
	}{
		{"left", keys.KeyLeft, focus.ButtonLeft, focus.DirLeft},
		{"right", keys.KeyRight, focus.ButtonRight, focus.DirRight},
		{"up", keys.KeyUp, focus.ButtonUp, focus.DirUp},
		{"down", keys.KeyDown, focus.ButtonDown, focus.DirDown},
		{"enter", keys.KeyEnter, focus.ButtonSelect, focus.DirUnknown},
		{"escape", keys.KeyEscape, focus.ButtonMenu, focus.DirUnknown},
		{"backspace", keys.KeyBackspace, focus.ButtonMenu, focus.DirUnknown},
		{"space", keys.KeySpace, focus.ButtonPlayPause, focus.DirUnknown},
		{"p", keys.KeyP, focus.ButtonPlayPause, focus.DirUnknown},
		{"media play/pause", keys.KeyMediaPlayPause, focus.ButtonPlayPause, focus.DirUnknown},
		{"h is the system's", keys.KeyH, focus.ButtonUnknown, focus.DirUnknown},
		{"s is not Siri", keys.KeyS, focus.ButtonUnknown, focus.DirUnknown},
		{"tab", keys.KeyTab, focus.ButtonUnknown, focus.DirUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := focus.ButtonOf(tt.key)
			assert.Equal(t, tt.want, b)
			assert.Equal(t, tt.dir, b.Dir())
		})
	}
}

func TestPrototypeTimings(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 550*time.Millisecond, focus.HoldDuration) // F.holdMs, focus.js:10
	assert.Equal(t, 140*time.Millisecond, focus.PressedPulse) // focus.js:140
}

func TestNames(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "ArrowLeft", focus.ButtonLeft.String())
	assert.Equal(t, "menu", focus.ButtonMenu.String())
	assert.Equal(t, "playpause", focus.ButtonPlayPause.String())
	assert.Equal(t, "select", focus.ButtonSelect.String())
	assert.Equal(t, "up", focus.DirUp.String())
	assert.Equal(t, "unknown", focus.DirUnknown.String())
	assert.Equal(t, "unknown", focus.ButtonUnknown.String())
}
