//go:build tvos

package nav_test

import (
	"testing"

	"dancefitvn-app/internal/ui/tv/nav"

	"github.com/stretchr/testify/assert"
)

func r(id string) nav.Route { return nav.Route{Screen: id} }

// TestStackGoPopReplace follows tvGo and tvBack (index.html:730-731): a push
// remembers the screen showing, a root forgets everything, Pop returns to the
// last and says false when there is none.
func TestStackGoPopReplace(t *testing.T) {
	t.Parallel()
	s := nav.NewStack()
	_, ok := s.Current()
	assert.False(t, ok)
	assert.Empty(t, s.Screen())
	_, popped := s.Pop()
	assert.False(t, popped)

	s.Go(r("welcome"), true)
	s.Go(r("code"), false)
	assert.Equal(t, "code", s.Screen())
	assert.Equal(t, 1, s.Depth())

	s.Go(r("home"), true) // after a login: a root
	assert.Equal(t, "home", s.Screen())
	assert.Zero(t, s.Depth())

	s.Go(r("detail"), false)
	s.Go(r("player"), false)
	s.Replace(r("upnext")) // the end of a video: nothing pushed
	assert.Equal(t, "upnext", s.Screen())
	assert.Equal(t, 2, s.Depth())

	got, ok := s.Pop()
	assert.True(t, ok)
	assert.Equal(t, "detail", got.Screen)
	assert.Equal(t, "detail", s.Screen())
	got, ok = s.Pop()
	assert.True(t, ok)
	assert.Equal(t, "home", got.Screen)
	_, ok = s.Pop()
	assert.False(t, ok)
	assert.Equal(t, "home", s.Screen(), "nothing popped: the screen stays")
}

// TestStackKeepsParams: the route carries what the screen was opened with.
func TestStackKeepsParams(t *testing.T) {
	t.Parallel()
	s := nav.NewStack()
	s.Go(nav.Route{Screen: "home"}, true)
	s.Go(nav.Route{Screen: "detail", Params: "c_set35"}, false)
	s.Go(nav.Route{Screen: "player"}, false)
	got, _ := s.Pop()
	assert.Equal(t, "c_set35", got.Params)
}

// TestStackPanelClosesWithTheScreen: the panel flag is set by SetPanel and
// every change of screen closes it.
func TestStackPanelClosesWithTheScreen(t *testing.T) {
	t.Parallel()
	s := nav.NewStack()
	s.Go(r("home"), true)
	s.Go(r("player"), false)
	s.SetPanel(true)
	assert.True(t, s.Panel())
	s.Replace(r("upnext"))
	assert.False(t, s.Panel())
	s.Go(r("player"), false)
	s.SetPanel(true)
	s.Pop()
	assert.False(t, s.Panel())
	s.SetPanel(true)
	s.Go(r("home"), true)
	assert.False(t, s.Panel())
}

// TestStackDoesNotAllocate: pushing and popping once the stack has grown
// allocates nothing.
func TestStackDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	s := nav.NewStack()
	s.Go(r("home"), true)
	home, detail := r("detail"), r("player")
	step := func() {
		s.Go(home, false)
		s.Go(detail, false)
		s.SetPanel(true)
		s.Replace(home)
		s.Pop()
		s.Pop()
		s.Go(home, true)
	}
	step()
	assert.InDelta(t, 0, testing.AllocsPerRun(100, step), 0)
}
