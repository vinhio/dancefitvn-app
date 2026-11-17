// Adapted from jplayer-tv/e2e/storecheck/main_test.go at 781bee5.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dancefitvn-app/internal/progress"
	"dancefitvn-app/internal/store"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

func TestParseArgs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want Config
		err  bool
	}{
		{"none", nil, Config{Label: "unlabeled"}, false},
		{"fresh", []string{"1-fresh", "-expect=defaults", "-write=v1"},
			Config{Label: "1-fresh", Expect: ExpectDefaults, Write: "v1"}, false},
		{"purge", []string{"3-purge", "-expect=v2", "-from=primary", "-write=v3"},
			Config{Label: "3-purge", Expect: "v2", From: store.SlotPrimary, Write: "v3"}, false},
		{"corrupt primary", []string{"5", "-corrupt=primary", "-from=copy"},
			Config{Label: "5", Corrupt: store.SlotPrimary, From: store.SlotCopy}, false},
		{"flags first", []string{"-expect=v2", "2-relaunch"}, Config{Label: "2-relaunch", Expect: "v2"}, false},
		{"unknown flag", []string{"x", "-reset"}, Config{}, true},
		{"unknown flag with value", []string{"x", "-quality=1080"}, Config{}, true},
		{"bad slot", []string{"x", "-corrupt=both"}, Config{}, true},
		{"bad from", []string{"x", "-from=none"}, Config{}, true},
		{"empty value", []string{"x", "-write="}, Config{}, true},
		{"flag without value", []string{"x", "-expect"}, Config{}, true},
		{"two labels", []string{"x", "y"}, Config{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseArgs(c.args)
			if c.err {
				assert.ErrorIs(t, err, core.ErrInvalid)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

// TestCheckSequence runs the script's nine launches over two Dir slots,
// the system events played by hand: a launch is a new Mirror over the same
// folders; the purge empties the copy's folder (Library/Caches); the
// install over loses NSUserDefaults' last write (jPlayer TV T0.5); the
// reinstall starts from empty folders.
func TestCheckSequence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	slots := func() Slots {
		return Slots{
			Primary: store.NewDir(filepath.Join(dir, "primary")),
			Copy:    store.NewDir(filepath.Join(dir, "copy")),
		}
	}
	steps := []struct {
		args []string
		from string // the slot every read came from
	}{
		{[]string{"1-fresh", "-expect=defaults", "-write=v1"}, "none"},
		{[]string{"2-relaunch", "-expect=v1", "-from=primary", "-write=v2"}, "primary"},
		{[]string{"3-purge", "-expect=v2", "-from=primary", "-write=v3"}, "primary"},
		{[]string{"4-after-purge", "-expect=v3", "-from=primary", "-write=v4"}, "primary"},
		{[]string{"5-corrupt-primary", "-corrupt=primary", "-expect=v4", "-from=copy", "-write=v5"}, "copy"},
		{[]string{"6-corrupt-copy", "-corrupt=copy", "-expect=v5", "-from=primary", "-write=v6"}, "primary"},
		{[]string{"7-reboot", "-expect=v6", "-write=v7"}, "primary"},
		{[]string{"8-install-over", "-expect=v7", "-write=v8"}, "copy"},
		{[]string{"9-reinstall", "-expect=defaults"}, "none"},
	}
	afterStep6 := map[string][]byte{}
	for i, st := range steps {
		cfg, err := ParseArgs(st.args)
		require.NoError(t, err)
		switch i {
		case 2:
			require.NoError(t, os.RemoveAll(filepath.Join(dir, "copy")))
		case 7:
			for k, raw := range afterStep6 {
				require.NoError(t, slots().Primary.Save(k, raw))
			}
		case 8:
			dir = t.TempDir()
		default:
		}
		var out bytes.Buffer
		ok := Check(cfg, slots(), fakeID, &out)
		log := out.String()
		assert.True(t, ok, "step %s:\n%s", cfg.Label, log)
		assert.Contains(t, log, "tvstore: PASS "+cfg.Label+" ", log)
		for _, k := range Keys {
			assert.Regexp(t, "slots label="+cfg.Label+" key="+k+" seq=[0-9]+ from="+st.from+" ", log)
		}
		if i == 2 {
			assert.Contains(t, log, "key=prefs seq=2 from=primary primaryErr=nil copyErr=notfound(", log)
		}
		assert.NotContains(t, log, "slot error", log)
		if cfg.Write != "" {
			for _, k := range Keys {
				assert.Regexp(t, "wrote label="+cfg.Label+" key="+k+" id="+cfg.Write+` err=nil seq=\d+ from=primary readback=nil primaryErr=nil copyErr=nil`, log)
			}
			assert.Contains(t, log, "tvstore: sync label="+cfg.Label+" err=nil", log)
		}
		if i == 5 {
			for _, k := range Keys {
				raw, err := slots().Primary.Load(k)
				require.NoError(t, err)
				afterStep6[k] = raw
			}
		}
	}
}

func TestCheckFailsOnMismatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  Config
		want []string
	}{
		{"defaults", Config{Label: "x", Expect: ExpectDefaults}, []string{"FAIL x expect=defaults from=none: prefs", "queue lessons=v2", "device={"}},
		{"other id", Config{Label: "x", Expect: "v9"}, []string{"FAIL x expect=v9 from=none: prefs auto_next=false quality=720p scenario=v2", "queue lessons=v2", "device="}},
		{"other slot", Config{Label: "x", From: store.SlotCopy}, []string{"FAIL x expect= from=copy: prefs from=primary; progress-queue from=primary; device from=primary"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			slots := Slots{Primary: store.NewMemory(), Copy: store.NewMemory()}
			require.True(t, Check(Config{Label: "w", Write: "v2"}, slots, fakeID, &bytes.Buffer{}))
			var out bytes.Buffer
			assert.False(t, Check(c.cfg, slots, fakeID, &out))
			for _, w := range c.want {
				assert.Contains(t, out.String(), w)
			}
		})
	}
}

func TestCheckMissingIsNotDefaults(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	assert.False(t, Check(Config{Label: "x", Expect: "v1"}, Slots{Primary: store.NewMemory(), Copy: store.NewMemory()}, fakeID, &out))
	log := out.String()
	assert.Contains(t, log, "key=prefs auto_next=true quality=auto scenario= err=notfound(")
	assert.Contains(t, log, "key="+progress.QueueKey+" lessons= err=notfound(")
	assert.Contains(t, log, "FAIL x expect=v1")
}

func TestCheckCorruptsEveryKey(t *testing.T) {
	t.Parallel()

	slots := Slots{Primary: store.NewMemory(), Copy: store.NewMemory()}
	require.True(t, Check(Config{Label: "w", Write: "v2"}, slots, fakeID, &bytes.Buffer{}))
	var out bytes.Buffer
	ok := Check(Config{Label: "x", Corrupt: store.SlotPrimary, Expect: "v2", From: store.SlotCopy}, slots, fakeID, &out)
	log := out.String()
	assert.True(t, ok, log)
	for _, k := range Keys {
		assert.Contains(t, log, "corrupt label=x slot=primary key="+k+" err=nil")
		assert.Regexp(t, "slots label=x key="+k+" seq=1 from=copy primaryErr=invalid", log)
	}
}

func TestCheckWithoutExpectSkips(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	assert.True(t, Check(Config{Label: "x"}, Slots{Primary: store.NewMemory(), Copy: store.NewMemory()}, fakeID, &out))
	assert.Contains(t, out.String(), "tvstore: SKIP x no -expect")
	assert.Contains(t, out.String(), "tvstore: install label=x id=0123abcd err=nil")
	assert.NotContains(t, out.String(), "wrote")
}

func TestCheckReportsBadSlots(t *testing.T) {
	t.Parallel()

	primary, cp := store.NewMemory(), store.NewMemory()
	cp.SaveErr = core.ErrUnsupported
	var out bytes.Buffer
	assert.True(t, Check(Config{Label: "x", Expect: ExpectDefaults, Write: "v2"}, Slots{Primary: primary, Copy: cp}, fakeID, &out))
	log := out.String()
	assert.Contains(t, log, "slot error label=x slot=copy")
	assert.Contains(t, log, "wrote label=x key=prefs id=v2 err=nil seq=1 from=primary")
	assert.Equal(t, len(Keys), primary.Saves)
}

func TestSlotName(t *testing.T) {
	t.Parallel()

	for s, want := range map[store.Slot]string{
		store.SlotUnknown: "none", store.SlotPrimary: "primary", store.SlotCopy: "copy", store.Slot(9): "slot9",
	} {
		assert.Equal(t, want, SlotName(s))
	}
}

func TestErrText(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "nil", errText(nil))
	assert.True(t, strings.HasPrefix(errText(core.ErrNotFound), "notfound("))
	assert.True(t, strings.HasPrefix(errText(core.ErrInvalid), "invalid("))
	assert.Equal(t, core.ErrClosed.Error(), errText(core.ErrClosed))
}

func TestShortAndEmail(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "none", short(""))
	assert.Equal(t, "abc", short("abc"))
	assert.Equal(t, "0123abcd", short("0123abcd-ef"))
	assert.Equal(t, "v1@example.invalid", Email("v1"))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// fakeID is an install id for the Mac.
func fakeID() (string, error) { return "0123abcd-0000-4000-8000-000000000000", nil }
