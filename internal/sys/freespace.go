package sys

import "fmt"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// FreeSpace returns the bytes the app may still write on the volume
// holding path, an existing file or folder (the audio folder), and the
// volume's size, for sheet 09's check before a download and the storage
// bar of screen 11 (docs/plan/03 §9).
//
// On iOS free is NSURLVolumeAvailableCapacityForImportantUsageKey: what
// the system would free for a file the user asked for, purgeable space
// counted, and total NSURLVolumeTotalCapacityKey; statfs answers when
// Foundation does not, and on tvOS, which lacks the first key. On Android it is statfs (what
// StatFs.getAvailableBytes reads). Elsewhere, the Mac and the tests, the
// space is unknown: 0, 0 and core.ErrUnsupported (Q-P5-9). It does disk
// I/O and may run on any goroutine; it is no hot path.
//
// The iOS privacy manifest (assets/ios/PrivacyInfo.xcprivacy) declares
// both calls under NSPrivacyAccessedAPICategoryDiskSpace with two
// reasons: E174.1, checking there is room before writing a download
// (sheet 09), and 85F4.1, showing the user the free space (the storage
// bar of screen 11, K11). The TV shell links no FreeSpace, so the tvOS
// manifest declares no DiskSpace; scripts/release/check-ios.sh fails a
// build whose program uses a category its manifest lacks, and
// internal/release keeps the iOS declaration tied to freespace_ios.go.
func FreeSpace(path string) (free, total int64, err error) {
	free, total, err = freeSpace(path)
	if err != nil {
		return 0, 0, fmt.Errorf("sys: free space of %s: %w", path, err)
	}

	return free, total, nil
}
