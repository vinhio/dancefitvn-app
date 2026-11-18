//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openOptionsSheet opens the options sheet from the "more" button and lets it slide
// in.
func openOptionsSheet(t testing.TB, r *shelltest.Rig) {
	t.Helper()
	tap(r, named(r.Shell, optionsOpen, 1))
	r.Tester.Elapse(400 * time.Millisecond)
	require.True(t, r.Shell.Nav().Sheet().IsOpen())
}

// TestOptionsSheetBoxesMatchPrototype lays the options sheet out over screen 06 with
// the iPhone's real text engine and compares every element of M07 with the
// prototype's within 1 DIP: the dim, the panel and its grab, the header, the five
// speeds, the three rows with their icons and switches, the quality row, "Xong".
// The one exception is the width of the text "Tự động ›", which Core Text sets
// 1.04 narrower than Chrome does, so that the row's title and value move by that.
func TestOptionsSheetBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r, pb, _ := atVideo(t, kittest.Phones[0], true)
	pb.Pause()
	r.Settle()
	openOptionsSheet(t, r)
	lay := r.Dump("M07")
	skip := map[string]bool{"span.tt#4": true, "b#4": true, "span.val": true}
	checkAgainstProtoWithout(t, "M07", lay, 1, skip, videoSideDrop...)
	want := shelltest.Without(shelltest.ProtoLayout(t, "M07"), videoSideDrop...)
	for key := range skip {
		w, _ := want.Find(key)
		g, ok := lay.Find(key)
		require.True(t, ok, key)
		assert.InDelta(t, w.X, g.X, 1.5, key+": x")
		assert.InDelta(t, w.Y, g.Y, 1, key+": y")
		assert.InDelta(t, w.W, g.W, 1.5, key+": w")
		assert.InDelta(t, w.H, g.H, 1, key+": h")
	}
}

// TestOptionsSheetOnTheWiderPhone checks the 412 wide phone's sheet against the
// iPhone's: the panel is as wide as the window and rests on its bottom, the rows keep
// their order and heights, the controls span the 20 DIP margins.
func TestOptionsSheetOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	rI, pbI, _ := atVideo(t, kittest.Phones[0], true)
	rA, pbA, _ := atVideo(t, kittest.Phones[1], true)
	pbI.Pause()
	pbA.Pause()
	openOptionsSheet(t, rI)
	openOptionsSheet(t, rA)
	a, b := rI.Dump("M07"), rA.Dump("M07")
	pa, _ := a.Find("div.sheet")
	pb, _ := b.Find("div.sheet")
	assert.InDelta(t, 412, pb.W, .01)
	assert.InDelta(t, 892, pb.Y+pb.H, .01, "on the bottom of the window")
	assert.InDelta(t, pa.H, pb.H, .6, "as tall as on the iPhone")
	for _, k := range []string{"div.sh", "div.segc", "mirror", "loop", "beat", "quality", "closeSheet#3"} {
		ea, _ := a.Find(k)
		eb, _ := b.Find(k)
		assert.InDelta(t, 353+19, eb.W, .6, k+": spans the margins") // 412 less 2 x 20 is 372
		assert.InDelta(t, ea.H, eb.H, .6, k+": height")
		assert.InDelta(t, ea.Y-pa.Y, eb.Y-pb.Y, .6, k+": place in the sheet")
	}
	seg, _ := b.Find("div.segc")
	first, _ := b.Find("speed:0.5")
	last, _ := b.Find("speed:1.5")
	assert.InDelta(t, seg.X+4, first.X, .01)
	assert.InDelta(t, seg.X+seg.W-4, last.X+last.W, .01)
}
