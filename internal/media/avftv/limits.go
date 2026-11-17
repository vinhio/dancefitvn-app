// Adapted from jplayer-tv/internal/media/avf/limits.go at 781bee5.

package avftv

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// limitsOf is the item's preferredMaximumResolution (width × height, in
// pixels) and preferredPeakBitRate (bits per second) for q (KT7). By
// decision OQ-T4-3 only the resolution is limited: 1080 → 1920×1080,
// 720 → 1280×720, 480 → 854×480; Auto and Unknown are 0×0, which
// AVFoundation reads as no limit. The peak bit rate is always 0 (none).
func limitsOf(q Quality) (w, h, peakBPS float64) {
	switch q {
	case Quality1080:
		return 1920, 1080, 0
	case Quality720:
		return 1280, 720, 0
	case Quality480:
		return 854, 480, 0
	case QualityAuto, QualityUnknown:
		return 0, 0, 0
	default:
		return 0, 0, 0
	}
}
