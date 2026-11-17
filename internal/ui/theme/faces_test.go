// Adapted from jplayer-tv/internal/ui/theme/faces_test.go at 781bee5.

package theme_test

import (
	"encoding/binary"
	"io/fs"
	"path"
	"runtime"
	"slices"
	"strings"
	"testing"

	"dancefitvn-app/assets"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFaceMatchesCSSWeights(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		role   theme.Role
		weight text.Weight
		ps     string // the face under Core Text naming
		family string // the family under gotext naming
		gtW    text.Weight
	}{
		{"brand 800", theme.RoleDisplay, 800, "BarlowCondensed-ExtraBold", "Barlow Condensed", 800},
		{"display 700", theme.RoleDisplay, 700, "BarlowCondensed-Bold", "Barlow Condensed", 700},
		{"display 600 draws in 700, its nearest shipped", theme.RoleDisplay, 600, "BarlowCondensed-Bold", "Barlow Condensed", 700},
		{"display 900 matches 800", theme.RoleDisplay, 900, "BarlowCondensed-ExtraBold", "Barlow Condensed", 800},
		{"display 400 matches 600, then 700", theme.RoleDisplay, 400, "BarlowCondensed-Bold", "Barlow Condensed", 700},
		{"body 400", theme.RoleBody, 400, "Inter-Regular", "Inter", 400},
		{"body 500", theme.RoleBody, 500, "Inter-Medium", "Inter", 500},
		{"body 600", theme.RoleBody, 600, "Inter-SemiBold", "Inter", 600},
		{"body 700", theme.RoleBody, 700, "Inter-Bold", "Inter", 700},
		{"body 800", theme.RoleBody, 800, "Inter-ExtraBold", "Inter", 800},
		{"body 900 matches 800", theme.RoleBody, 900, "Inter-ExtraBold", "Inter", 800},
		{"body 300 matches 400", theme.RoleBody, 300, "Inter-Regular", "Inter", 400},
		{"body 450 matches 500", theme.RoleBody, 450, "Inter-Medium", "Inter", 500},
		{"body 0 reads as 400", theme.RoleBody, 0, "Inter-Regular", "Inter", 400},
		{"unknown role reads as body", theme.RoleUnknown, 700, "Inter-Bold", "Inter", 700},
		{"role out of range reads as body", theme.Role(99), 600, "Inter-SemiBold", "Inter", 600},
		{"mono 500", theme.RoleMono, 500, "JetBrainsMono-Medium", "JetBrains Mono", 500},
		{"mono 700", theme.RoleMono, 700, "JetBrainsMono-Bold", "JetBrains Mono", 700},
		{"mono 400 matches 500", theme.RoleMono, 400, "JetBrainsMono-Medium", "JetBrains Mono", 500},
		{"mono 600 matches 700", theme.RoleMono, 600, "JetBrainsMono-Bold", "JetBrains Mono", 700},
		{"mono 800 matches 700", theme.RoleMono, 800, "JetBrainsMono-Bold", "JetBrains Mono", 700},
	}
	ct, gt := theme.NewFaces(theme.NamingCoreText), theme.NewFaces(theme.NamingGoText)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, text.Face{Family: tc.ps, Size: 15, Weight: text.WeightRegular}, ct.Face(tc.role, 15, tc.weight),
				"Core Text: the PostScript name at regular weight, the file being the weight")
			assert.Equal(t, text.Face{Family: tc.family, Size: 15, Weight: tc.gtW}, gt.Face(tc.role, 15, tc.weight),
				"gotext: the family at the shipped weight")
		})
	}
}

func TestNaming(t *testing.T) {
	t.Parallel()

	want := theme.NamingGoText
	if runtime.GOOS == "darwin" || runtime.GOOS == "ios" {
		want = theme.NamingCoreText
	}
	assert.Equal(t, want, theme.DefaultNaming())
	tests := []struct {
		name string
		in   theme.Naming
		want theme.Naming
	}{
		{"core text", theme.NamingCoreText, theme.NamingCoreText},
		{"gotext", theme.NamingGoText, theme.NamingGoText},
		{"unknown reads as the default", theme.NamingUnknown, want},
		{"out of range reads as the default", theme.Naming(42), want},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, theme.NewFaces(tc.in).Naming())
		})
	}
	assert.Equal(t, want, theme.Faces{}.Naming(), "the zero Faces")
}

func TestFamily(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Barlow Condensed", theme.Family(theme.RoleDisplay))
	assert.Equal(t, "Inter", theme.Family(theme.RoleBody))
	assert.Equal(t, "Inter", theme.Family(theme.RoleUnknown))
	assert.Equal(t, "JetBrains Mono", theme.Family(theme.RoleMono))
}

// TestEveryFaceShips checks that each PostScript name Face can return is a
// file of assets.Fonts whose own PostScript name is that name, and that
// each shipped font is a face.
func TestEveryFaceShips(t *testing.T) {
	t.Parallel()

	names := theme.FaceNames()
	require.Len(t, names, 9, "PLAN D5: Barlow Condensed 700, 800; Inter 400-800; JetBrains Mono 500, 700")
	for _, n := range names {
		data, err := fs.ReadFile(assets.Fonts, "fonts/"+n+".ttf")
		require.NoError(t, err, n)
		ps, ok := postScriptName(data)
		require.True(t, ok, n)
		assert.Equal(t, n, ps, "the file's name table")
	}
	files, err := fs.Glob(assets.Fonts, "fonts/*.ttf")
	require.NoError(t, err)
	shipped := make([]string, 0, len(files))
	for _, f := range files {
		shipped = append(shipped, strings.TrimSuffix(path.Base(f), ".ttf"))
	}
	slices.Sort(shipped)
	want := slices.Clone(names)
	slices.Sort(want)
	assert.Equal(t, want, shipped, "every shipped font is a face and every face ships")
}

func TestFaceDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	ct, gt := theme.NewFaces(theme.NamingCoreText), theme.NewFaces(theme.NamingGoText)

	assert.Zero(t, testing.AllocsPerRun(100, func() { _ = ct.Face(theme.RoleDisplay, 72, 800) }), "core text")
	assert.Zero(t, testing.AllocsPerRun(100, func() { _ = gt.Face(theme.RoleBody, 15, 600) }), "gotext")
}

func BenchmarkFace(b *testing.B) {
	f := theme.NewFaces(theme.NamingCoreText)
	b.ReportAllocs()
	for b.Loop() {
		_ = f.Face(theme.RoleBody, 15, 600)
	}
}

// postScriptName reads name ID 6 of an sfnt font from its name table: the
// platform 3 (Windows, UTF-16BE) record, else the platform 1 (Mac) one.
func postScriptName(data []byte) (string, bool) {
	if len(data) < 12 {
		return "", false
	}
	tables := int(binary.BigEndian.Uint16(data[4:]))
	for i := range tables {
		rec := data[12+i*16:]
		if string(rec[:4]) != "name" {
			continue
		}
		off := int(binary.BigEndian.Uint32(rec[8:]))
		tbl := data[off:]
		count := int(binary.BigEndian.Uint16(tbl[2:]))
		strOff := int(binary.BigEndian.Uint16(tbl[4:]))
		var mac string
		for j := range count {
			r := tbl[6+j*12:]
			platform, id := binary.BigEndian.Uint16(r), binary.BigEndian.Uint16(r[6:])
			length, start := int(binary.BigEndian.Uint16(r[8:])), int(binary.BigEndian.Uint16(r[10:]))
			if id != 6 {
				continue
			}
			raw := tbl[strOff+start : strOff+start+length]
			switch platform {
			case 3:
				var b strings.Builder
				for k := 0; k+1 < len(raw); k += 2 {
					b.WriteRune(rune(binary.BigEndian.Uint16(raw[k:])))
				}

				return b.String(), true
			case 1:
				mac = string(raw)
			default:
			}
		}

		return mac, mac != ""
	}

	return "", false
}
