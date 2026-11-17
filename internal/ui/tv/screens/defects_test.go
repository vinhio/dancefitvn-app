//go:build tvos

package screens_test

import (
	"strings"
	"testing"

	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// longName is a course named as production names them.
const longName = `DanceFIT® Set 35: "Cùng nhảy cùng hát và một tên rất dài nữa"`

// TestDetailLongNameWrapsInsideItsColumn is F2: the title is a block of the 780 DIP
// column, wrapped in at most two lines, so that it neither runs over the lesson list at
// 960 nor pushes the buttons off the 1080 stage.
func TestDetailLongNameWrapsInsideItsColumn(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewSignedServices(t)
	svc.CourseData.Name = longName
	r := openDetail(t, svc, false)
	r.settle()
	lay := r.layout()
	d, ok := lay.Find("div.d")
	require.True(t, ok)
	assert.LessOrEqual(t, d.X+d.W, 90+780+.5, "inside the column")
	assert.LessOrEqual(t, d.X+d.W, 960.0, "short of the lesson list")
	assert.Greater(t, d.H, 84.0, "wrapped on a second line")
	assert.LessOrEqual(t, d.H, 2*84.0+2, "two lines at most")
	d1, ok := lay.Find("d1")
	require.True(t, ok)
	assert.LessOrEqual(t, d1.Y+d1.H, 1080.0, "the buttons stay on the stage")
}

// TestDetailRowsOfLessonsWithoutATag is F3 on TV06: the row's line is "04:32", with no
// stray separator, and "04:32 · Đã xem" for a lesson watched.
func TestDetailRowsOfLessonsWithoutATag(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewSignedServices(t)
	for i := range svc.CourseData.Lessons {
		svc.CourseData.Lessons[i].Tag = ""
	}
	svc.MarkWatched(svc.CourseData.Lessons[0].ID)
	r := openDetail(t, svc, false)
	r.settle()
	for _, s := range r.tt.Shown() {
		assert.False(t, strings.HasPrefix(s, " · "), "no stray separator: %q", s)
		assert.False(t, strings.HasSuffix(s, " · "), "no stray separator: %q", s)
	}
	assert.True(t, strings.Contains(strings.Join(r.tt.Shown(), "\n"), "· Đã xem"))
}

// TestDetailPosterAsksForTheCover is K14 on TV06: the poster asks for the course's
// cover_url; the vector poster stays where it is refused (the mock's), and the picture
// covers the poster once it is loaded.
func TestDetailPosterAsksForTheCover(t *testing.T) {
	t.Parallel()
	const url = "https://cdn.example.test/covers/set-35.jpg"
	green, err := scene.NewImage(8, 8)
	require.NoError(t, err)
	for i := 0; i < len(green.Pix); i += 4 {
		green.Pix[i], green.Pix[i+1], green.Pix[i+2], green.Pix[i+3] = 0, 255, 0, 255
	}

	refused := tvtest.NewSignedServices(t)
	refused.CourseData.CoverURL = url
	r := openDetail(t, refused, false)
	r.settle()
	assert.Equal(t, []string{url}, refused.CoverCalls())
	vector := r.tt.Snapshot().RGBAAt(470, 500)
	assert.NotEqual(t, uint8(255), vector.G, "the vector poster when the cover is refused")

	loaded := tvtest.NewSignedServices(t)
	loaded.CourseData.CoverURL = url
	loaded.SetCover(url, green)
	r = openDetail(t, loaded, false)
	r.settle()
	px := r.tt.Snapshot().RGBAAt(470, 334)
	assert.Equal(t, uint8(255), px.G, "the loaded cover covers the poster")
	assert.Zero(t, px.R)
}

// TestPlayerWithoutATagHasNoEmptyChip is the empty tag chip on TV07: a lesson with no tag
// shows no yellow pill, and "Video dọc 9:16" starts the row at the block's left, with no
// gap before it; a tagged lesson is as it was.
func TestPlayerWithoutATagHasNoEmptyChip(t *testing.T) {
	t.Parallel()
	tagged := inPlayer(t, tvtest.NewPlayerServices(t), false)
	lay := tagged.layout()
	tag, ok := lay.Find("span.chip.y")
	require.True(t, ok, "a lesson with a tag has its chip")
	kind, ok := lay.Find("span.chip")
	require.True(t, ok)
	assert.Greater(t, kind.X, tag.X+tag.W, "the kind chip follows the tag chip and the gap")

	svc := tvtest.NewPlayerServices(t)
	svc.PB.(*tvtest.Playback).LessonV.Tag = ""
	bare := inPlayer(t, svc, false)
	lay2 := bare.layout()
	_, ok = lay2.Find("span.chip.y")
	assert.False(t, ok, "no empty tag chip")
	kind2, ok := lay2.Find("span.chip")
	require.True(t, ok)
	assert.InDelta(t, tag.X, kind2.X, .01, "the kind chip takes the first place")
	assert.InDelta(t, kind.W, kind2.W, .01)
	assert.InDelta(t, kind.Y, kind2.Y, .01, "the rows below keep their place")
}
