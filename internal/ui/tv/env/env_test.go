//go:build tvos

package env_test

import (
	"testing"

	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/stretchr/testify/assert"
)

func TestFillAndT(t *testing.T) {
	t.Parallel()
	e := env.New(tvtest.NewServices(), map[string]string{"a": "Mã hết hạn sau {t} · {x}", "b": "plain"})
	assert.Equal(t, "plain", e.T("b"))
	assert.Equal(t, "missing.key", e.T("missing.key"), "a missing string shows as its key")
	assert.Equal(t, "Mã hết hạn sau 04:32 · y", e.Fill("a", "t", "04:32", "x", "y"))
	assert.Equal(t, "plain", e.Fill("b", "t", "1"))
	assert.Equal(t, "Mã hết hạn sau {t} · {x}", e.Fill("a"), "no values: the template")
	assert.NotNil(t, e.Stack)
}

func TestTabsAreTheNavItems(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"home", "courses", "music", "settings"}, env.Tabs[:])
}

// TestPrefsFallback: services without preferences give the defaults of a fresh
// app and cycle the quality Tự động, 1080p, 720p, Tự động; a stored 480p goes to
// Tự động (Q-P7-19).
func TestPrefsFallback(t *testing.T) {
	t.Parallel()
	e := env.New(tvtest.NewServices(), nil)
	p := e.Prefs()
	assert.True(t, p.AutoNext())
	assert.Equal(t, store.QualityAuto, p.Quality())
	for _, want := range []store.Quality{store.QualityP1080, store.QualityP720, store.QualityAuto, store.QualityP1080} {
		p.CycleQualityTV()
		assert.Equal(t, want, p.Quality())
	}
	p.SetAutoNext(false)
	assert.False(t, e.Prefs().AutoNext(), "the same in-memory Prefs each time")
}

// TestFillDropsTheSeparatorOfAnEmptyValue is F3: a lesson with no tag leaves no stray
// " · " in the hero's meta line, the up-next line or the lesson rows; with the tag the
// text is the template filled, byte for byte.
func TestFillDropsTheSeparatorOfAnEmptyValue(t *testing.T) {
	t.Parallel()
	e := env.New(tvtest.NewServices(), map[string]string{
		"meta": "{course} · BÀI {nn} / {n} · {tag}", "row": "{tag} · {dur}", "next": "{tag} · {dur} · video + file âm thanh",
	})
	assert.Equal(t, "SET 35 · BÀI 01 / 10 · Warm-up", e.Fill("meta", "course", "SET 35", "nn", "01", "n", "10", "tag", "Warm-up"))
	assert.Equal(t, "SET 35 · BÀI 01 / 10", e.Fill("meta", "course", "SET 35", "nn", "01", "n", "10", "tag", ""))
	assert.Equal(t, "Warm-up · 04:32", e.Fill("row", "tag", "Warm-up", "dur", "04:32"))
	assert.Equal(t, "04:32", e.Fill("row", "tag", "", "dur", "04:32"))
	assert.Equal(t, "04:32 · video + file âm thanh", e.Fill("next", "tag", "", "dur", "04:32"))
}
