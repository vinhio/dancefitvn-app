package nav_test

import (
	"testing"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/nav"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// block is a widget of the width it gets and a height.
type block struct {
	ui.Base
	h float32
}

// Layout takes the width it gets and its height.
func (b *block) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(geom.Sz(cs.Max.W, b.h))
}

// screen is a Page the rig counts the leaving of.
type screen struct {
	*nav.Page
	id  string
	rig *rig
}

// Left counts the screen's leaving.
func (s *screen) Left() { s.rig.left[s.id]++ }

// tabOf is the tab each test screen is the root of: -1 for the others.
var tabOf = map[string]int{"home": 0, "courses": 1, "downloads": 2, "account": 3}

// rig is a Nav of test screens mounted on a phone.
type rig struct {
	n     *nav.Nav
	tt    *ui.Tester
	built []string       // the screens the factory built, in order
	left  map[string]int // how many times a screen was left
	last  map[string]*screen
}

// route is a Route of the screen id.
func route(id string) nav.Route { return nav.Route{Screen: id} }

// tabs are TABS (index.html:323) with their routes.
func tabs() [kit.TabCount]nav.TabRoute {
	return [kit.TabCount]nav.TabRoute{
		{Item: kit.TabItem{Icon: icon.Home, Label: "Trang chủ", Name: "go.home"}, Route: route("home")},
		{Item: kit.TabItem{Icon: icon.Grid, Label: "Khoá học", Name: "go.courses"}, Route: route("courses")},
		{Item: kit.TabItem{Icon: icon.Dl, Label: "Tải xuống", Name: "go.downloads"}, Route: route("downloads")},
		{Item: kit.TabItem{Icon: icon.User, Label: "Tài khoản", Name: "go.account"}, Route: route("account")},
	}
}

// newRig mounts a Nav on ph, at Home as the first root.
func newRig(t testing.TB, ph kittest.Phone) *rig { return newRigWith(t, ph, nil) }

// newRigWith is newRig with content: it builds the content of a screen by its id,
// nil for a tall block.
func newRigWith(t testing.TB, ph kittest.Phone, content func(id string) ui.Widget) *rig {
	t.Helper()
	r := &rig{left: map[string]int{}, last: map[string]*screen{}}
	r.n = nav.New(nav.Options{
		Home: route("home"),
		Tabs: tabs(),
		Factory: func(rt nav.Route) nav.Screen {
			r.built = append(r.built, rt.Screen)
			tab, ok := tabOf[rt.Screen]
			if !ok {
				tab = -1
			}
			var w ui.Widget = &block{h: 2000}
			if content != nil {
				w = content(rt.Screen)
			}
			s := &screen{Page: nav.NewPage(w, tab), id: rt.Screen, rig: r}
			r.last[rt.Screen] = s

			return s
		},
	})
	r.n.Go(route("home"), true)
	r.tt = kittest.Mount(t, ph, r.n)
	kittest.PinMotion(r.tt)
	r.tt.Elapse(kittest.Frame)

	return r
}

// at returns the ids of the stack, bottom first.
func (r *rig) stack() []string {
	out := []string{}
	for _, rt := range r.n.Stack() {
		out = append(out, rt.Screen)
	}

	return out
}

// tapBlock is a block of the width it gets that counts the presses on it.
type tapBlock struct {
	ui.Pressable
	h    float32
	taps int
}

// newTapBlock returns a tapBlock of height h.
func newTapBlock(h float32) *tapBlock {
	b := &tapBlock{h: h}
	b.OnPress = func() { b.taps++ }

	return b
}

// Layout takes the width it gets and its height.
func (b *tapBlock) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(geom.Sz(cs.Max.W, b.h))
}
