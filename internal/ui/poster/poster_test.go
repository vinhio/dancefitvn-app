package poster_test

import (
	"encoding/json"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"testing"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/poster"

	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCourses reads the courses of the mock API (courses/<id>.json), in the
// order of the file names.
func mockCourses(t testing.TB) []model.Course {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(moduleRoot(t), "mock", "dancefitvn-mock-api", "courses", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	var out []model.Course
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)
		var c model.Course
		require.NoError(t, json.Unmarshal(data, &c))
		out = append(out, c)
	}

	return out
}

// TestPosterTitleBoxesMatchPrototype lays out the titles of the 156x100,
// 80x80 and 72x72 posters on both phones and compares div.ttl, s and b with the
// boxes of K-posters.layout.json: left 8 %, bottom 8 %, the line boxes Chrome
// makes (heights exact, widths within the engines' advance difference).
func TestPosterTitleBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	lay := readLayout(t, "local/shots/prototype-kit/mobile/K-posters.layout.json")
	cases := []struct {
		poster, ttl, s, b string
		pp                protoPoster
	}{
		{"div.poster#2", "div.ttl", "s", "b", kPosters[1]},
		{"div.poster#3", "div.ttl#2", "s#2", "b#2", kPosters[2]},
		{"div.poster#4", "div.ttl#3", "s#3", "b#3", kPosters[3]},
		{"div.poster#6", "div.ttl#4", "s#4", "b#4", kPosters[5]},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range cases {
			px, py, pw, ph0, _, _ := lay.box(t, tc.poster)
			p := poster.New(tc.pp.spec, float32(pw), float32(ph0))
			root := &stage{}
			root.put(p, 10, 10)
			tt := kittest.MountReal(t, ph, root)
			check := func(name, key string, widthTol float64) {
				x, y, w, h, _, _ := lay.box(t, key)
				_, r, ok := tt.Find(name)
				require.True(t, ok, name)
				assert.InDelta(t, x-px, float64(r.X)-10, 1, "%s x", key)
				assert.InDelta(t, y-py, float64(r.Y)-10, 1, "%s y", key)
				assert.InDelta(t, h, float64(r.H), 1, "%s h", key)
				assert.InDelta(t, w, float64(r.W), widthTol, "%s w", key)
			}
			check("div.ttl", tc.ttl, 1.5)
			check("s", tc.s, 1.5)
			check("b", tc.b, 1.5)
		}
	})
}

// TestPosterBehavesOnBothPhones mounts posters on both phones with the fake
// shaper and checks the tree: names, the title block, hidden title, extra,
// size from the room given, radius.
func TestPosterBehavesOnBothPhones(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		full := poster.New(poster.Spec{ID: "set-35", Title: "SET 35", Radius: poster.RadiusDefault}, 0, 232)
		bare := poster.New(poster.Spec{ID: "hot-extra", Radius: 0, NoTitle: true}, 100, 100)
		marker := &ui.Box{Width: 20, Height: 20}
		withExtra := poster.New(poster.Spec{ID: "set-34", Radius: 10, NoTitle: true, NoFigure: true}, 44, 44)
		withExtra.SetExtra(marker)

		root := &stage{}
		root.put(full, 0, 0)
		root.put(bare, 0, 300)
		root.put(withExtra, 200, 300)
		tt := kittest.Mount(t, ph, root)

		assert.Equal(t, "div.poster", full.Name)
		assert.Equal(t, poster.DefaultRadius, full.Radius())
		assert.Zero(t, bare.Radius())
		assert.InDelta(t, ph.Size.W, full.Frame().W, .001, "width 0 takes the room given")
		assert.InDelta(t, 232, full.Frame().H, .001)

		for _, n := range []string{"div.ttl", "s", "b"} {
			_, _, ok := tt.Find(n)
			assert.True(t, ok, n)
		}
		_, r, _ := tt.Find("div.ttl")
		assert.InDelta(t, float64(full.Frame().W)*.08, r.X, .01, "left 8 %")
		assert.InDelta(t, 232-232*.08, r.Y+r.H, .01, "bottom 8 %")

		// The bare poster has no title in its tree view.
		for c := range bare.Children() {
			assert.True(t, c.Node().Hidden, "title hidden")
		}

		// The extra sits in the middle.
		assert.InDelta(t, 12, marker.Frame().X, .01)
		assert.InDelta(t, 12, marker.Frame().Y, .01)

		// A new spec relays the title out.
		full.SetSpec(poster.Spec{ID: "set-35", Title: "SET 35", Radius: 5, TitleSize: 30})
		tt.Elapse(kittest.Frame)
		_, r2, _ := tt.Find("b")
		assert.Greater(t, r2.H, float32(28), "ts 30 at .95")
		assert.InDelta(t, 5, full.Radius(), .001)
	})
}

// TestPosterImageCoversTheBox shows a cover image in place of the drawing: no
// title on top, the picture covering the box.
func TestPosterImageCoversTheBox(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		img, err := scene.NewImage(40, 20)
		require.NoError(t, err)
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 0, 255, 0, 255
		}
		p := poster.New(poster.Spec{ID: "set-35", Title: "SET 35", Radius: 0}, 100, 100)
		p.SetImage(img)
		root := &stage{}
		root.put(p, 10, 10)
		tt := kittest.Mount(t, ph, root)
		snap := tt.Snapshot()
		for _, pt := range []image.Point{{11, 11}, {109, 109}, {60, 60}, {12, 108}} {
			c := snap.RGBAAt(pt.X, pt.Y)
			assert.Equal(t, uint8(255), c.G, "%v", pt)
			assert.Zero(t, c.R, "%v", pt)
		}
		p.SetImage(nil)
		tt.Elapse(kittest.Frame)
		assert.NotEqual(t, uint8(255), tt.Snapshot().RGBAAt(60, 60).G)
	})
}

// TestPosterPaintDoesNotAllocate paints the dancers, the vinyl, the image and
// the extra after a warm-up, and counts nothing.
func TestPosterPaintDoesNotAllocate(t *testing.T) {
	// Not parallel: AllocsPerRun.
	for _, ph := range kittest.Phones {
		t.Run(ph.Name, func(t *testing.T) {
			root := &stage{}
			root.put(poster.New(poster.Spec{ID: "set-35", Title: "SET 35", Radius: 20, TitleSize: 20}, 156, 100), 10, 10)
			root.put(poster.New(poster.Spec{ID: "hot-extra-2", Title: "HOT EXTRA 2", Radius: 16}, 156, 100), 10, 120)
			root.put(poster.New(poster.Spec{ID: "set-24", Radius: 12, NoTitle: true}, 80, 80), 10, 230)
			tt := kittest.Mount(t, ph, root)
			for range 3 {
				tt.Elapse(kittest.Frame)
			}
			allocs := testing.AllocsPerRun(20, func() {
				root.Repaint()
				tt.Elapse(kittest.Frame)
			})
			assert.Zero(t, allocs, "steady frame")
			before := root.paints
			root.Repaint()
			tt.Elapse(kittest.Frame)
			assert.Greater(t, root.paints, before, "the frame was drawn")
		})
	}
}

// BenchmarkPosterPaint paints one poster, and so exercises the painter.
func BenchmarkPosterPaint(b *testing.B) {
	root := &stage{}
	root.put(poster.New(poster.Spec{ID: "set-35", Title: "SET 35", Radius: 20, TitleSize: 20}, 156, 100), 10, 10)
	tt := kittest.Mount(b, kittest.Phones[0], root)
	b.ReportAllocs()
	for b.Loop() {
		root.Repaint()
		tt.Elapse(kittest.Frame)
	}
}

// TestMockCoursesSheet draws the posters of every mock course at 156x100 and
// 353x232 on both phones, with the short names the API's names give, into
// local/shots/app-kit/posters, and checks the short names of the prototype.
func TestMockCoursesSheet(t *testing.T) {
	t.Parallel()
	courses := mockCourses(t)
	short := map[string]string{}
	for _, c := range courses {
		short[c.ID] = poster.ForCourse(c).Title
	}
	assert.Equal(t, "SET 35", short["set-35"])
	assert.Equal(t, "HOT EXTRA", short["hot-extra"])
	assert.Equal(t, "HOT EXTRA 2", short["hot-extra-2"])
	assert.Equal(t, "GIỚI THIỆU", short["intro"])

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		root := &stage{}
		for i, c := range courses {
			s := poster.ForCourse(c)
			s.TitleSize = 20
			root.put(poster.New(s, 156, 100), float32(4+(i%2)*160), float32(4+(i/2)*104))
		}
		tt := kittest.MountReal(t, ph, root)
		tt.Elapse(kittest.Frame)
		rows := (len(courses) + 1) / 2
		writePNG(t, "courses-"+ph.Name, crop(tt.Snapshot(), 0, 0, 324, 4+rows*104))

		root2 := &stage{}
		root2.put(poster.New(poster.ForCourse(courses[0]), 353, 232), 20, 20)
		tt2 := kittest.MountReal(t, ph, root2)
		tt2.Elapse(kittest.Frame)
		out := image.NewRGBA(image.Rect(0, 0, 353, 232))
		draw.Draw(out, out.Bounds(), crop(tt2.Snapshot(), 20, 20, 373, 252), image.Point{}, draw.Src)
		writePNG(t, "course-353x232-"+ph.Name, out)
	})
}

// TestPosterTitleComesBack checks the title of a poster that had none and gets one
// again (Spec.NoTitle flipped both ways): it is laid out and painted every time.
func TestPosterTitleComesBack(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		p := poster.New(poster.Spec{ID: "set-35", Title: "SET 35", Radius: 5}, 0, 232)
		tt := kittest.Mount(t, ph, p)
		_, first, ok := tt.Find("div.ttl")
		require.True(t, ok)
		require.Positive(t, first.H)

		for range 2 {
			p.SetSpec(poster.Spec{ID: "set-35", Title: "SET 35", Radius: 5, NoTitle: true})
			tt.Elapse(kittest.Frame)
			_, _, ok = tt.Find("div.ttl")
			assert.False(t, ok, "no title: nothing of it is drawn")

			p.SetSpec(poster.Spec{ID: "set-35", Title: "SET 35", Radius: 5})
			tt.Elapse(kittest.Frame)
			_, r, ok := tt.Find("div.ttl")
			require.True(t, ok, "the title is back on the screen")
			assert.InDelta(t, first.H, r.H, .01)
			assert.InDelta(t, first.X, r.X, .01)
			assert.True(t, tt.Shows("SET 35"))
		}
	})
}

// TestPosterIsDecoration checks a poster, with its title painted in, is no element
// of a screen reader's reading, on both phones: the course is named by what holds
// the poster, not by the picture.
func TestPosterIsDecoration(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		p := poster.New(poster.Spec{ID: "set-35", Title: "SET 35"}, 200, 200)
		root := &ui.Box{Width: 200, Height: 200}
		root.Add(p)
		tt := kittest.Mount(t, ph, root)
		assert.Len(t, tt.Semantics().Nodes, 1, "only the window")
	})
}
