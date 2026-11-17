//go:build !tvos

package screens_test

import (
	"strings"
	"testing"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// longCourse is a course named as production names them, with a lesson that has no tag.
func longCourse() (model.Course, []model.Lesson) {
	c := model.Course{
		ID: "set-35", Name: `DanceFIT® Set 35: "Cùng nhảy cùng hát và một tên rất dài nữa"`, LessonCount: 10,
		CoverURL: "https://cdn.example.test/covers/set-35.jpg",
	}
	l := model.Lesson{ID: "l_3501", No: 1, Title: "Khởi động", DurationSec: 272, HasAudio: true, AudioSizeBytes: 1 << 20}

	return c, []model.Lesson{l}
}

// withLongCourse is a setup that leaves one downloaded file, of the long course.
func withLongCourse(d *shelltest.FakeDownloads) {
	d.Clear()
	c, ls := longCourse()
	if err := d.Start(c, ls); err != nil {
		panic(err)
	}
	d.FinishDownload("l_3501")
}

// TestDownloadsGroupHeaderEllipsizesTheNameAndKeepsTheCount is F10 on both phones: the
// eyebrow of a group whose course name is longer than the column ends inside the
// column, the name cut with an ellipsis and "· 1/10 BÀI" whole after it; a row of a
// lesson with no tag reads "04:32", not "· 04:32".
func TestDownloadsGroupHeaderEllipsizesTheNameAndKeepsTheCount(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDownloads(t, ph, true, withLongCourse)
		lay := r.Dump("long")
		var head struct{ x, w float64 }
		found := false
		for _, e := range lay.Elements {
			if strings.HasSuffix(e.Text, "1/10 BÀI") {
				head.x, head.w, found = e.X, e.W, true
				assert.Contains(t, e.Text, "DANCEFIT® SET 35", "the dump keeps the whole line")
			}
		}
		require.True(t, found, "the group's eyebrow")
		assert.GreaterOrEqual(t, head.x, 20-.5)
		assert.LessOrEqual(t, head.x+head.w, float64(ph.Size.W)-20+.5, "the eyebrow ends inside the column")

		c, _ := longCourse()
		_, nameBox, ok := r.Tester.Find(c.Name)
		require.True(t, ok, "the name label")
		_, tailBox, ok := r.Tester.Find(" · 1/10 bài")
		require.True(t, ok, "the count label")
		assert.LessOrEqual(t, nameBox.X+nameBox.W, tailBox.X+.5, "the name is cut to what the count leaves")
		assert.LessOrEqual(t, tailBox.X+tailBox.W, float32(ph.Size.W)-20+.5, "the count is whole, inside the column")
		assert.True(t, r.Tester.Shows("04:32"), "a lesson with no tag: no stray separator")
	})
}

// TestDownloadsPosterShowsTheRealCover shows the cover a download kept the address
// of (K14, live): the poster asks for it, the vector poster stays while it is not
// there (the mock's answer), and once it is loaded the picture covers the 44 poster.
func TestDownloadsPosterShowsTheRealCover(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		c, _ := longCourse()
		r := atDownloads(t, ph, false, withLongCourse)
		r.Settle()
		assert.Contains(t, r.Fake.CoverCalls(), c.CoverURL, "the row asked for the course's cover")
		posterAt := func() (int, int) {
			for _, e := range r.Dump("p").Elements {
				if strings.HasPrefix(e.Key, "div.poster") && e.W == 44 {
					return int(e.X + e.W/2), int(e.Y + e.H/2)
				}
			}
			require.FailNow(t, "no 44 poster")

			return 0, 0
		}
		x, y := posterAt()
		assert.NotEqual(t, uint8(255), r.Tester.Snapshot().RGBAAt(x, y).G, "the vector poster until the cover is there")

		img, err := scene.NewImage(8, 8)
		require.NoError(t, err)
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 0, 255, 0, 255
		}
		r.Fake.SetCover(c.CoverURL, img)
		r.Shell.Go(nav.Route{Screen: env.RouteHome}, true) // leave, and come back: the screen is built again
		r.Settle()
		r.Shell.Go(nav.Route{Screen: env.RouteDownloads}, true)
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		r.Tester.Elapse(kittest.Frame)
		x, y = posterAt()
		px := r.Tester.Snapshot().RGBAAt(x, y)
		assert.Equal(t, uint8(255), px.G, "the loaded cover covers the poster")
		assert.Zero(t, px.R)
	})
}
