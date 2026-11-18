package store_test

import (
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/store"
)

func TestPrefsDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		wifiOnly bool
	}{
		{"wifi only by default", true},
		{"config turns wifi only off", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := store.OpenPrefs(store.NewMemory(), tt.wifiOnly)
			assert.Equal(t, store.Prefs{WiFiOnly: tt.wifiOnly, AutoNext: true, Quality: store.QualityAuto, Speed: 1}, s.Get())
			assert.NoError(t, s.LoadErr())
		})
	}
}

func TestPrefsPersistAcrossOpen(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s := store.OpenPrefs(store.NewDir(dir), true)
	require.NoError(t, s.Update(func(p *store.Prefs) {
		p.WiFiOnly = false
		p.AutoNext = false
		p.Quality = store.QualityP720
		p.Speed, p.Mirror, p.Beat = 1.25, true, true
		p.Scenarios = map[string]string{"me.get": "token_expired"}
	}))

	again := store.OpenPrefs(store.NewDir(dir), true)
	require.NoError(t, again.LoadErr())
	assert.Equal(t, store.Prefs{
		WiFiOnly: false, AutoNext: false, Quality: store.QualityP720,
		Speed: 1.25, Mirror: true, Beat: true,
		Scenarios: map[string]string{"me.get": "token_expired"},
	}, again.Get(), "the stored wifi_only wins over the config default")
}

func TestPrefsFileFormat(t *testing.T) {
	t.Parallel()

	b := store.NewMemory()
	s := store.OpenPrefs(b, true)
	require.NoError(t, s.Update(func(p *store.Prefs) { p.Quality = store.QualityP480 }))
	raw, err := b.Load("prefs")
	require.NoError(t, err)
	assert.JSONEq(t, `{"wifi_only":true,"auto_next":true,"quality":"480p","speed":1,"mirror":false,"beat":false}`, string(raw))
}

func TestPrefsMissingFieldsKeepDefaults(t *testing.T) {
	t.Parallel()

	b := store.NewMemory()
	b.Put("prefs", []byte(`{"quality":"1080p","future":1}`))
	p := store.OpenPrefs(b, true).Get()
	assert.True(t, p.WiFiOnly)
	assert.True(t, p.AutoNext)
	assert.Equal(t, store.QualityP1080, p.Quality)
	assert.InDelta(t, 1, p.Speed, 0, "K41: no speed stored is 1x")
	assert.False(t, p.Mirror)
	assert.False(t, p.Beat)
}

func TestPrefsSpeedOutOfRangeIsDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want float64
	}{
		{"kept", `{"speed":0.5}`, 0.5},
		{"top kept", `{"speed":1.5}`, 1.5},
		{"zero", `{"speed":0}`, 1},
		{"negative", `{"speed":-2}`, 1},
		{"too fast", `{"speed":9}`, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := store.NewMemory()
			b.Put("prefs", []byte(tt.raw))
			assert.InDelta(t, tt.want, store.OpenPrefs(b, true).Get().Speed, 0)
		})
	}
}

func TestPrefsBadRecord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want store.Quality
	}{
		{"garbage", "not json", store.QualityAuto},
		{"unknown quality text", `{"quality":"4k"}`, store.QualityAuto},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := store.NewMemory()
			b.Put("prefs", []byte(tt.raw))
			s := store.OpenPrefs(b, true)
			assert.Equal(t, tt.want, s.Get().Quality)
			assert.True(t, s.Get().AutoNext)
		})
	}
	b := store.NewMemory()
	b.Put("prefs", []byte("not json"))
	assert.ErrorIs(t, store.OpenPrefs(b, true).LoadErr(), core.ErrInvalid)
	b.LoadErr = assert.AnError
	assert.ErrorIs(t, store.OpenPrefs(b, true).LoadErr(), assert.AnError)
}

func TestPrefsUpdateSaveFailure(t *testing.T) {
	t.Parallel()

	b := store.NewMemory()
	b.SaveErr = assert.AnError
	s := store.OpenPrefs(b, true)
	require.ErrorIs(t, s.Update(func(p *store.Prefs) { p.AutoNext = false }), assert.AnError)
	assert.False(t, s.Get().AutoNext, "kept in memory")
}

func TestPrefsGetReturnsCopy(t *testing.T) {
	t.Parallel()

	s := store.OpenPrefs(store.NewMemory(), true)
	require.NoError(t, s.Update(func(p *store.Prefs) { p.Scenarios = map[string]string{"a": "b"} }))
	g := s.Get()
	g.Scenarios["a"] = "changed"
	assert.Equal(t, "b", s.Get().Scenarios["a"])
}

func TestQualityCycles(t *testing.T) {
	t.Parallel()

	mobile := []store.Quality{store.QualityAuto, store.QualityP1080, store.QualityP720, store.QualityP480, store.QualityAuto}
	for i := range len(mobile) - 1 {
		assert.Equal(t, mobile[i+1], mobile[i].NextMobile(), "mobile %v", mobile[i])
	}
	tv := []store.Quality{store.QualityAuto, store.QualityP1080, store.QualityP720, store.QualityAuto}
	for i := range len(tv) - 1 {
		assert.Equal(t, tv[i+1], tv[i].NextTV(), "tv %v", tv[i])
	}
	assert.Equal(t, store.QualityAuto, store.QualityP480.NextTV(), "480p is not on the TV cycle")
	assert.Equal(t, store.QualityP1080, store.QualityUnknown.NextMobile(), "unknown reads as auto")
	assert.Equal(t, store.QualityP1080, store.QualityUnknown.NextTV())
}

func TestQualityText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		q      store.Quality
		text   string
		height int
	}{
		{store.QualityAuto, "auto", 0},
		{store.QualityP1080, "1080p", 1080},
		{store.QualityP720, "720p", 720},
		{store.QualityP480, "480p", 480},
		{store.QualityUnknown, "", 0},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.text, tt.q.String())
		assert.Equal(t, tt.height, tt.q.Height())
		if tt.q != store.QualityUnknown {
			var back store.Quality
			require.NoError(t, back.UnmarshalText([]byte(tt.text)))
			assert.Equal(t, tt.q, back)
		}
	}
}
