package qr

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pairingURL = "https://dancefitvn.com/tv?c=7K4M2P"

var allLevels = []Level{LevelL, LevelM, LevelQ, LevelH}

func levelName(l Level) string { return [5]string{"?", "L", "M", "Q", "H"}[l] }

func filler(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*31 + i/7)
	}
	return b
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		data        []byte
		lvl         Level
		wantVersion int
		wantSize    int
	}{
		{"pairing url", []byte(pairingURL), LevelM, 3, 29},
		{"empty", nil, LevelM, 1, 21},
		{"one byte", []byte("A"), LevelM, 1, 21},
		{"v1-M edge 14", filler(14), LevelM, 1, 21},
		{"v2-M 15", filler(15), LevelM, 2, 25},
		{"v3-M edge 42", filler(42), LevelM, 3, 29},
		{"v4-M 43", filler(43), LevelM, 4, 33},
		{"200 bytes", filler(200), LevelM, 10, 57},
		{"binary all values", filler(256), LevelM, 12, 65},
		{"zeros", make([]byte, 60), LevelM, 4, 33},
		{"pairing url L", []byte(pairingURL), LevelL, 3, 29},
		{"pairing url H", []byte(pairingURL), LevelH, 4, 33},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := Encode(tt.data, tt.lvl)
			require.NoError(t, err)
			assert.Equal(t, tt.wantVersion, c.Version())
			assert.Equal(t, tt.wantSize, c.Size())
			d, err := decode(gridOf(c))
			require.NoError(t, err)
			assert.Equal(t, tt.data == nil, len(d.data) == 0)
			if len(tt.data) > 0 {
				assert.Equal(t, tt.data, d.data)
			}
			assert.Equal(t, c.Version(), d.version)
			assert.Equal(t, tt.lvl, d.level)
			assert.Equal(t, c.Mask(), d.mask)
		})
	}
}

// TestEncodeEveryVersionAndLevel fills each version to capacity (and to
// one byte over the previous version's capacity) at every level.
func TestEncodeEveryVersionAndLevel(t *testing.T) {
	t.Parallel()
	for v := 1; v <= MaxVersion; v++ {
		for _, l := range allLevels {
			t.Run(fmt.Sprintf("v%d-%s", v, levelName(l)), func(t *testing.T) {
				t.Parallel()
				lens := []int{byteCapacity(v, l)}
				if v > 1 {
					lens = append(lens, byteCapacity(v-1, l)+1)
				}
				for _, n := range lens {
					data := filler(n)
					c, err := Encode(data, l)
					require.NoError(t, err)
					assert.Equal(t, v, c.Version())
					d, err := decode(gridOf(c))
					require.NoError(t, err)
					assert.Equal(t, data, d.data)
				}
			})
		}
	}
}

func TestEncodeErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
		lvl  Level
	}{
		{"unknown level", []byte("x"), LevelUnknown},
		{"level out of range", []byte("x"), Level(9)},
		{"too long M", make([]byte, byteCapacity(MaxVersion, LevelM)+1), LevelM},
		{"too long H", make([]byte, byteCapacity(MaxVersion, LevelH)+1), LevelH},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := Encode(tt.data, tt.lvl)
			assert.Nil(t, c)
			assert.ErrorIs(t, err, core.ErrInvalid)
		})
	}
	assert.Equal(t, 2331, byteCapacity(40, LevelM))
	assert.Equal(t, 2953, byteCapacity(40, LevelL))
}

func TestBlackOutside(t *testing.T) {
	t.Parallel()
	c, err := Encode([]byte(pairingURL), LevelM)
	require.NoError(t, err)
	assert.False(t, c.Black(-1, 0))
	assert.False(t, c.Black(0, c.Size()))
	assert.True(t, c.Black(0, 0))  // finder corner
	assert.False(t, c.Black(7, 0)) // separator
	assert.Equal(t, c.Size()*(c.Size()+1), len(c.String()))
}

func TestDecoderRejectsDamage(t *testing.T) {
	t.Parallel()
	c, err := Encode([]byte(pairingURL), LevelM)
	require.NoError(t, err)
	g := grid{size: c.size, dark: append([]bool(nil), c.dark...)}
	g.dark[20*g.size+12] = !g.dark[20*g.size+12] // a data module
	_, err = decode(g)
	assert.Error(t, err)
}

func TestConcurrentEncode(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	want, err := Encode([]byte(pairingURL), LevelM)
	require.NoError(t, err)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := Encode([]byte(pairingURL), LevelM)
			assert.NoError(t, err)
			assert.Equal(t, want.String(), c.String())
		}()
	}
	wg.Wait()
}

func readCases(t *testing.T) (names []string) {
	t.Helper()
	ins, err := filepath.Glob("testdata/*.in")
	require.NoError(t, err)
	require.NotEmpty(t, ins)
	for _, in := range ins {
		names = append(names, strings.TrimSuffix(filepath.Base(in), ".in"))
	}
	return names
}

func loadCase(t *testing.T, name string, l Level) (data []byte, theirs grid) {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name + ".in")
	require.NoError(t, err)
	txt, err := os.ReadFile("testdata/" + name + "." + levelName(l) + ".txt")
	require.NoError(t, err)
	theirs, err = gridFromText(string(txt))
	require.NoError(t, err)
	return data, theirs
}

// TestDecoderReadsQRencode proves the test decoder against a standard encoder.
func TestDecoderReadsQRencode(t *testing.T) {
	t.Parallel()
	for _, name := range readCases(t) {
		for _, l := range allLevels {
			t.Run(name+"."+levelName(l), func(t *testing.T) {
				t.Parallel()
				data, theirs := loadCase(t, name, l)
				d, err := decode(theirs)
				require.NoError(t, err)
				assert.Equal(t, data, d.data)
				assert.Equal(t, l, d.level)
			})
		}
	}
}

// TestMatchesQRencode compares matrices with libqrencode. When only the
// mask differs, the matrix must still equal ours forced to their mask.
func TestMatchesQRencode(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var maskDiffs []string
	t.Run("cases", func(t *testing.T) {
		for _, name := range readCases(t) {
			for _, l := range allLevels {
				t.Run(name+"."+levelName(l), func(t *testing.T) {
					t.Parallel()
					data, theirs := loadCase(t, name, l)
					ours, err := Encode(data, l)
					require.NoError(t, err)
					require.Equal(t, theirs.size, ours.Size(), "version")
					if bytes.Equal(boolBytes(theirs.dark), boolBytes(ours.dark)) {
						return
					}
					d, err := decode(theirs)
					require.NoError(t, err)
					forced, err := encode(data, l, 1, d.mask)
					require.NoError(t, err)
					assert.Equal(t, theirs.dark, forced.dark, "differs beyond the mask")
					mu.Lock()
					maskDiffs = append(maskDiffs, fmt.Sprintf("%s.%s ours=%d theirs=%d", name, levelName(l), ours.Mask(), d.mask))
					mu.Unlock()
				})
			}
		}
	})
	if len(maskDiffs) > 0 {
		t.Logf("mask choice differs from libqrencode in %d cases: %v", len(maskDiffs), maskDiffs)
	}
}

func boolBytes(b []bool) []byte {
	out := make([]byte, len(b))
	for i, v := range b {
		if v {
			out[i] = 1
		}
	}
	return out
}

// TestForcedMasksMatchQRencode checks every mask of the pairing URL decodes.
func TestForcedMasks(t *testing.T) {
	t.Parallel()
	for m := 0; m < 8; m++ {
		t.Run(fmt.Sprint(m), func(t *testing.T) {
			t.Parallel()
			c, err := encode([]byte(pairingURL), LevelM, 1, m)
			require.NoError(t, err)
			assert.Equal(t, m, c.Mask())
			d, err := decode(gridOf(c))
			require.NoError(t, err)
			assert.Equal(t, pairingURL, string(d.data))
		})
	}
}

// renderPNG draws c with scale pixels per module and a 4-module quiet zone.
func renderPNG(c *Code, scale int) image.Image {
	const quiet = 4
	side := (c.Size() + 2*quiet) * scale
	img := image.NewGray(image.Rect(0, 0, side, side))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	for y := 0; y < c.Size(); y++ {
		for x := 0; x < c.Size(); x++ {
			if !c.Black(x, y) {
				continue
			}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					img.SetGray((x+quiet)*scale+dx, (y+quiet)*scale+dy, color.Gray{})
				}
			}
		}
	}
	return img
}

// TestCoreImageDecodes writes PNGs and reads them back with CIDetector.
// Run with DFV_SLOW=1 (the swift script compiles on every run).
func TestCoreImageDecodes(t *testing.T) {
	if os.Getenv("DFV_SLOW") != "1" {
		t.Skip("set DFV_SLOW=1 to run the CoreImage check")
	}
	swift, err := exec.LookPath("swift")
	if err != nil {
		t.Skip("swift not on PATH")
	}
	script, err := filepath.Abs("testdata/coreimage.swift")
	require.NoError(t, err)
	dir := t.TempDir()
	tests := []struct {
		name string
		data string
		lvl  Level
	}{
		{"pairing", pairingURL, LevelM},
		{"pairing-L", pairingURL, LevelL},
		{"pairing-H", pairingURL, LevelH},
		{"v10", string(filler(0)) + strings.Repeat("dancefit ", 22), LevelM},
	}
	for _, tt := range tests {
		c, err := Encode([]byte(tt.data), tt.lvl)
		require.NoError(t, err)
		path := filepath.Join(dir, tt.name+".png")
		f, err := os.Create(path)
		require.NoError(t, err)
		require.NoError(t, png.Encode(f, renderPNG(c, 8)))
		require.NoError(t, f.Close())
		out, err := exec.Command(swift, script, path).CombinedOutput()
		require.NoError(t, err, string(out))
		assert.Equal(t, tt.data, strings.TrimSuffix(string(out), "\n"), tt.name)
	}
}
