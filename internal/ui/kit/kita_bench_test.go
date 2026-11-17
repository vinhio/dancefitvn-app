package kit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
)

// benchRepaint mounts items and measures a steady repaint.
func benchRepaint(b *testing.B, items ...stageItem) {
	b.Helper()
	tt := mountStage(b, kittest.Phones[0], items...)
	tt.Elapse(kittest.Frame)
	tt.Repaint()
	b.ReportAllocs()
	for b.Loop() {
		tt.Repaint()
	}
}

// BenchmarkInputRepaint is a steady repaint of an input with a placeholder.
func BenchmarkInputRepaint(b *testing.B) {
	benchRepaint(b, placeAt(kit.NewInput(icon.Mail, "Email"), 20, 100, 353))
}

// BenchmarkRowRepaint is a steady repaint of a row with a sub line and a switch.
func BenchmarkRowRepaint(b *testing.B) {
	r := kit.NewRow(icon.Mirror, "Lật gương", "Tập theo như đứng trước gương")
	r.Trailing = kit.NewSwitch(true)
	benchRepaint(b, placeAt(r, 20, 100, 353))
}

// BenchmarkSwitchRepaint is a steady repaint of a switch.
func BenchmarkSwitchRepaint(b *testing.B) { benchRepaint(b, placeAt(kit.NewSwitch(true), 20, 100, 0)) }

// BenchmarkSegmentedRepaint is a steady repaint of a five button control.
func BenchmarkSegmentedRepaint(b *testing.B) {
	benchRepaint(b, placeAt(kit.NewSegmented("0.5x", "0.75x", "1x", "1.25x", "1.5x"), 20, 100, 353))
}

// BenchmarkRadioRepaint is a steady repaint of a radio.
func BenchmarkRadioRepaint(b *testing.B) {
	benchRepaint(b, placeAt(kit.NewRadio("Chỉ bài này", "05 · Hãy Hát Lên", "3,1 MB"), 20, 100, 353))
}

// BenchmarkChipRepaint is a steady repaint of a chip.
func BenchmarkChipRepaint(b *testing.B) {
	benchRepaint(b, placeAt(kit.NewChip(kit.ChipTh, "THƯỜNG · 10 BÀI"), 20, 100, 0))
}

// BenchmarkProgressRepaint is a steady repaint of a bar.
func BenchmarkProgressRepaint(b *testing.B) {
	benchRepaint(b, placeAt(&kit.Progress{Value: .45}, 20, 100, 257))
}
