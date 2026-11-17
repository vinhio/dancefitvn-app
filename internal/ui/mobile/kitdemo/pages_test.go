//go:build !tvos

package kitdemo_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"testing/fstest"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/mobile/kitdemo"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dump mounts the page on the phone with the fake shaper, lets it settle and
// returns what layoutdump records.
func dump(t *testing.T, ph kittest.Phone, p kitdemo.Page, d *kitdemo.Data) (*ui.Tester, layoutdump.Layout) {
	t.Helper()
	s := p.Build(d)
	tt := kittest.Mount(t, ph, s.Root)
	kittest.PinMotion(tt)
	tt.Elapse(600 * time.Millisecond)

	return tt, layoutdump.Dump(tt.View().Context(), tt.View().Root(), p.ID, "mobile", ph.Size, s.Skip)
}

// TestEveryTextKeyIsInTheLocale checks that every key a page asks of the data
// is in locales/vi.json: a missing one would show as the key itself.
func TestEveryTextKeyIsInTheLocale(t *testing.T) {
	t.Parallel()

	texts := locales.MustTexts()
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	re := regexp.MustCompile(`\bd\.T\("([A-Za-z0-9_.]+)"\)`)
	found := 0
	for _, f := range files {
		if filepath.Ext(f) != ".go" || regexp.MustCompile(`_test\.go$`).MatchString(f) {
			continue
		}
		src, err := os.ReadFile(f)
		require.NoError(t, err)
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			found++
			_, ok := texts[m[1]]
			assert.True(t, ok, "%s: key %q is not in locales/vi.json", f, m[1])
		}
	}
	assert.Greater(t, found, 50, "the scan found the keys")
}

// TestPagesBuildOnBothPhones builds every phone page on both phones and checks
// that it has named elements, each of a sane size and place (a hidden page or a
// widget at NaN would show here).
func TestPagesBuildOnBothPhones(t *testing.T) {
	t.Parallel()

	d := loadData(t)
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, p := range kitdemo.Pages() {
			if p.TV {
				continue
			}
			t.Run(p.ID, func(t *testing.T) {
				_, lay := dump(t, ph, p, d)
				require.NotEmpty(t, lay.Elements)
				for _, e := range lay.Elements {
					assert.GreaterOrEqual(t, e.W, 0.0, e.Key)
					assert.GreaterOrEqual(t, e.H, 0.0, e.Key)
					assert.Less(t, e.X, 2000.0, "%s starts at %v", e.Key, e.X) // the home strip runs past the screen
				}
			})
		}
	})
}

// TestPagesFollowTheStretchRule checks, on the 412 wide phone, the boxes the
// 07 section 3.3 rule gives: a full-width button grows by 19, a right-anchored
// button moves by 19, the tab bar and the pinned button follow the bottom.
func TestPagesFollowTheStretchRule(t *testing.T) {
	t.Parallel()

	d := loadData(t)
	ios, android := kittest.Phones[0], kittest.Phones[1]
	m16, _ := kitdemo.Find("M16-code")
	m03, _ := kitdemo.Find("M03")
	_, a := dump(t, ios, m16, d)
	_, b := dump(t, android, m16, d)
	for _, key := range []string{"linkTV", "input", "div.top"} {
		ea, ok := a.Find(key)
		require.True(t, ok, key)
		eb, ok := b.Find(key)
		require.True(t, ok, key)
		assert.InDelta(t, ea.X, eb.X, 0.01, key+" keeps its left margin")
		assert.InDelta(t, ea.W+19, eb.W, 0.01, key+" stretches")
	}
	ea, _ := a.Find("linkTV")
	eb, _ := b.Find("linkTV")
	assert.InDelta(t, ea.Y+40, eb.Y, 0.01, "the pinned button follows the bottom edge")

	_, a = dump(t, ios, m03, d)
	_, b = dump(t, android, m03, d)
	for key, dx := range map[string]float64{"go.account": 19, "go.courses": 19} {
		ea, ok := a.Find(key)
		require.True(t, ok, key)
		eb, ok := b.Find(key)
		require.True(t, ok, key)
		assert.InDelta(t, ea.X+dx, eb.X, 0.01, key+" moves with the right edge")
		assert.InDelta(t, ea.W, eb.W, 0.01, key+" keeps its width")
	}
	ta, _ := a.Find("div.tabs")
	tb, _ := b.Find("div.tabs")
	assert.InDelta(t, ta.W+19, tb.W, 0.01)
	assert.InDelta(t, ta.Y+40, tb.Y, 0.01)
}

// protoLayout reads the prototype capture of a state, skipping the test without
// local/.
func protoLayout(t *testing.T, state string) layoutdump.Layout {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "local", "shots", "prototype-kit", "mobile", state+".layout.json"))
	if err != nil {
		t.Skipf("prototype capture not available: %v", err)
	}
	var lay layoutdump.Layout
	require.NoError(t, json.Unmarshal(data, &lay))

	return lay
}

// TestPagePlacesKitWidgetsAtThePrototypeBoxes checks, on the iPhone with Core
// Text, a handful of boxes per page against the prototype's layout.json: the
// widgets the kit already matches within 1 pt (the full table is cmd/shotcmp's).
func TestPagePlacesKitWidgetsAtThePrototypeBoxes(t *testing.T) {
	t.Parallel()

	d := loadData(t)
	ph := kittest.Phones[0]
	for _, tc := range []struct {
		page string
		keys []string
	}{
		{"M02", []string{"div.inp", "div.inp#2", "button.btn.p", "bioLogin", "pwEye"}},
		{"M02B", []string{"div.inp.err", "div.help", "button.btn.p"}},
		{"M03", []string{"go.detail:set35", "go.detail:set31", "span.chip.y", "div.tabs", "go.home", "go.account#2"}},
		{"M04", []string{"div.segc", "filter:Thường", "div.inp", "span.chip.c-th", "span.chip.c-gt"}},
		{"M05", []string{"playVideo:set35-1", "playVideo:set35-3#2", "dlTap:set35-3", "dlAll:set35"}},
		{"M07", []string{"div.sheet", "speed:0.75", "mirror#2", "span.sw.on", "closeSheet#3"}},
		{"M09", []string{"div.sheet", "dlChoice:one", "dlChoice:all", "pref:wifi", "dlConfirm"}},
		{"M13", []string{"div.offbar", "offTap:set35-3", "playAudio:set35-1"}},
		{"M14", []string{"div.sheet", "delConfirm", "closeSheet#2"}},
		{"M15", []string{"sheet:avatar", "div.badge", "go.info", "pref:wifi", "span.sw.on#2", "sheet:logout"}},
		{"M15-logout", []string{"div.sheet", "logout", "closeSheet#2"}},
		{"M15B", []string{"div.top", "div.inp", "div.inp#2", "div.inp#3", "button.btn.p"}},
		{"M16", []string{"div.top", "span.codebox.f", "linkTV"}},
		{"M16-code", []string{"span.codebox.f", "span.codebox#5", "linkTV"}},
		{"K-icons", []string{"svg.i", "svg.i#42"}},
	} {
		t.Run(tc.page, func(t *testing.T) {
			p, ok := kitdemo.Find(tc.page)
			require.True(t, ok)
			proto := protoLayout(t, tc.page)
			s := p.Build(d)
			tt := kittest.MountReal(t, ph, s.Root)
			kittest.PinMotion(tt)
			tt.Elapse(600 * time.Millisecond)
			app := layoutdump.Dump(tt.View().Context(), tt.View().Root(), tc.page, "mobile", ph.Size, s.Skip)
			for _, key := range tc.keys {
				want, ok := proto.Find(key)
				require.True(t, ok, "the prototype has %s", key)
				got, ok := app.Find(key)
				require.True(t, ok, "the page has %s", key)
				assert.InDelta(t, want.X, got.X, 1.6, key+" x")
				assert.InDelta(t, want.Y, got.Y, 1.6, key+" y")
				assert.InDelta(t, want.W, got.W, 1.6, key+" w")
				assert.InDelta(t, want.H, got.H, 1.6, key+" h")
			}
		})
	}
}

// TestSheetPagesOpenAndCloseTheirSheet checks that the sheet pages show their
// sheet on the first frames and that the sheet's own buttons close it.
func TestSheetPagesOpenAndCloseTheirSheet(t *testing.T) {
	t.Parallel()

	d := loadData(t)
	for _, tc := range []struct{ page, closeBy string }{
		{"M07", "Xong"}, {"M09", "Đóng"}, {"M14", "Huỷ"}, {"M15-logout", "Huỷ"},
	} {
		t.Run(tc.page, func(t *testing.T) {
			t.Parallel()
			kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
				p, _ := kitdemo.Find(tc.page)
				tt, lay := dump(t, ph, p, d)
				_, ok := lay.Find("div.sheet")
				require.True(t, ok, "the sheet is open")
				_, _, found := tt.Find(tc.closeBy)
				if tc.closeBy == "Đóng" {
					// the close button is an icon: its label is the aria one
					_, _, found = tt.Find("Đóng")
				}
				require.True(t, found, "the sheet has %q", tc.closeBy)
				require.NoError(t, tt.Click(tc.closeBy))
				tt.Elapse(kittest.Frame)
				lay = layoutdump.Dump(tt.View().Context(), tt.View().Root(), tc.page, "mobile", ph.Size, nil)
				_, ok = lay.Find("div.sheet")
				assert.False(t, ok, "%q closes the sheet", tc.closeBy)
			})
		})
	}
}

// TestToastPageShowsTheMessageFor2200ms checks M-toast: the toast is on the
// screen at 1 s and at 2.1 s and gone at 2.3 s (the kit's own test pins the
// 2200 ms to the frame).
func TestToastPageShowsTheMessageFor2200ms(t *testing.T) {
	t.Parallel()

	d := loadData(t)
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		p, _ := kitdemo.Find("M-toast")
		s := p.Build(d)
		tt := kittest.Mount(t, ph, s.Root)
		kittest.PinMotion(tt)
		tt.Elapse(time.Second)
		assert.True(t, tt.Shows("Đã tải xong bài 03"), "the toast reads Đã tải xong bài 03")
		tt.Elapse(1100 * time.Millisecond)
		assert.True(t, tt.Shows("Đã tải xong bài 03"), "still showing at 2.1 s")
		tt.Elapse(200 * time.Millisecond)
		assert.False(t, tt.Shows("Đã tải xong bài 03"))
	})
}

// TestGalleryOpensAPageAndBackComesBack checks the gallery: the list names the
// phone pages, a tap opens one, the Back key returns (taking its press and its
// release), and Back on the list is left to the system.
func TestGalleryOpensAPageAndBackComesBack(t *testing.T) {
	t.Parallel()

	d := loadData(t)
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		g := kitdemo.NewGallery(d, "")
		tt := kittest.Mount(t, ph, g)
		for _, p := range kitdemo.Pages() {
			_, _, ok := tt.Find(p.ID)
			assert.Equal(t, !p.TV, ok, "%s is in the list, unless it is a TV page", p.ID)
		}

		require.NoError(t, tt.Click("M02"))
		tt.Elapse(kittest.Frame)
		assert.True(t, tt.Shows("Quên mật khẩu?"), "the login page is open")
		assert.True(t, tt.Key(keys.KeyBack, 0), "Back is taken on a page")
		tt.Elapse(kittest.Frame)
		_, _, ok := tt.Find("M03")
		assert.True(t, ok, "Back returned to the list")
		assert.False(t, tt.Key(keys.KeyBack, 0), "Back is left to the system on the list")
		assert.False(t, g.Back())
		assert.False(t, g.Open("TV05"), "a TV page has no place on a phone")
		assert.False(t, g.Open("nope"))
		assert.True(t, g.Open("M13"))
	})
}

// TestGalleryOpensThePageOfTheEnvironment checks the DFV_KIT path: a page
// named at creation is open at once.
func TestGalleryOpensThePageOfTheEnvironment(t *testing.T) {
	t.Parallel()

	d := loadData(t)
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		tt := kittest.Mount(t, ph, kitdemo.NewGallery(d, "M13"))
		tt.Elapse(kittest.Frame)
		assert.True(t, tt.Shows(d.T("offline.bar")))
	})
}

// TestLoadReportsWhatIsWrong checks the errors of Load: no folder, a missing
// file, a file that is not JSON.
func TestLoadReportsWhatIsWrong(t *testing.T) {
	t.Parallel()

	_, err := kitdemo.Load(nil, nil)
	require.ErrorIs(t, err, core.ErrInvalid)

	_, err = kitdemo.Load(fstest.MapFS{}, nil)
	require.ErrorIs(t, err, fs.ErrNotExist)

	mock := os.DirFS(filepath.Join(repoRoot(t), "mock", "dancefitvn-mock-api"))
	broken := fstest.MapFS{}
	for _, name := range []string{"courses/set-35.json", "me/home.json", "me/me.json", "me/courses.json"} {
		data, rerr := fs.ReadFile(mock, name)
		require.NoError(t, rerr)
		broken[name] = &fstest.MapFile{Data: data}
	}
	broken["me/me.json"] = &fstest.MapFile{Data: []byte("{")}
	_, err = kitdemo.Load(broken, nil)
	require.ErrorIs(t, err, core.ErrInvalid)

	d, err := kitdemo.Load(mock, map[string]string{"k": "v"})
	require.NoError(t, err)
	assert.Equal(t, "v", d.T("k"))
	assert.Equal(t, "missing.key", d.T("missing.key"), "a missing key shows as itself")
	assert.Len(t, d.Set35.Lessons, 10)
	assert.Equal(t, "set35", kitdemo.ProtoID("set-35"))
	assert.Equal(t, "hotextra2", kitdemo.ProtoID("hot-extra-2"))
	assert.True(t, kitdemo.Downloaded(6))
	assert.False(t, kitdemo.Downloaded(7))
}

// TestKitDemoPageStaysSteady checks that a page at rest paints a frame with no
// allocation: the gallery adds nothing to what the kit keeps at zero.
func TestKitDemoPageStaysSteady(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	d := loadData(t)
	p, _ := kitdemo.Find("M05")
	tt := kittest.Mount(t, kittest.Phones[0], p.Build(d).Root)
	tt.Elapse(time.Second)
	allocs := testing.AllocsPerRun(20, func() {
		tt.Repaint()
		tt.Frame()
	})
	assert.Zero(t, allocs, "a steady frame of M05 allocates nothing")
}
