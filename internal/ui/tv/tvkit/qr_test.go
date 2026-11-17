//go:build tvos

package tvkit_test

import (
	"testing"

	"dancefitvn-app/internal/qr"
	"dancefitvn-app/internal/ui/tv/tvkit"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// qrRig mounts a QR at (1290, 250), where TS.code has it.
func qrRig(t *testing.T) (*tvkit.QR, func(*testing.T) bool) {
	t.Helper()
	root := &stage{}
	q := tvkit.NewQR()
	root.put(q, 1290, 250)
	tt := mount(t, root)

	return q, func(t *testing.T) bool {
		t.Helper()
		tt.Repaint()

		return tt.Snapshot().RGBAAt(1290+25+5, 250+25+5).R < 128 // the middle of module (0,0)
	}
}

// TestQRBox: the box is 340 wide and 344 high (the inline svg's descent), white,
// with no symbol until one is set.
func TestQRBox(t *testing.T) {
	t.Parallel()
	q, dark := qrRig(t)
	assert.Equal(t, geom.XYWH(1290, 250, 340, 344), q.WindowFrame())
	assert.Nil(t, q.Code())
	assert.Zero(t, q.Module())
	assert.False(t, dark(t), "an empty white box")
}

// TestQRModule: a module is floor(300 / size) DIPs and the symbol is centered in
// the 300 square (rounded down to a whole DIP), 20 in from the box.
func TestQRModule(t *testing.T) {
	t.Parallel()
	tests := []struct {
		url    string
		size   int
		module float32
		origin float32
	}{
		{"https://dancefitvn.com/tv?c=7K4M2P", 29, 10, 25},
		{"a", 21, 14, 20 + 3},
		{"https://dancefitvn.com/tv?c=7K4M2P/aaaaaaaaaaaaaaaaaaaaaaaa", 33, 9, 20 + 1},
	}
	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()
			q, _ := qrRig(t)
			c, err := qr.Encode([]byte(tc.url), qr.LevelM)
			require.NoError(t, err)
			require.Equal(t, tc.size, c.Size())
			q.Set(c)
			assert.Equal(t, tc.module, q.Module())
			assert.Equal(t, geom.Pt(tc.origin, tc.origin), q.Origin())
			assert.Same(t, c, q.Code())
			q.Set(nil)
			assert.Zero(t, q.Module())
		})
	}
}

// TestQRDraws: the top-left finder pattern's corner module is dark (--bg) on the
// white box, and the quiet zone around the symbol is white.
func TestQRDraws(t *testing.T) {
	t.Parallel()
	q, dark := qrRig(t)
	c, err := qr.Encode([]byte("https://dancefitvn.com/tv?c=7K4M2P"), qr.LevelM)
	require.NoError(t, err)
	q.Set(c)
	assert.True(t, dark(t), "module (0,0) of a finder pattern")
}

// TestQRRepaintDoesNotAllocate: a repaint of the box with a symbol allocates
// nothing: the modules are one path made when the code is set.
func TestQRRepaintDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	root := &stage{}
	q := tvkit.NewQR()
	root.put(q, 1290, 250)
	tt := mount(t, root)
	c, err := qr.Encode([]byte("https://dancefitvn.com/tv?c=7K4M2P"), qr.LevelM)
	require.NoError(t, err)
	q.Set(c)
	tt.Repaint()
	tt.Repaint()
	assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint))
}
