// Adapted from jplayer-mobile/internal/media/avf/stats_ios.go at 49de3c2.

//go:build ios && !tvos && cgo

package avf

import (
	"sync"
	"unsafe"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// vmInfo is the buffer task_info fills with a task_vm_info (mach/task_info.h:
// 335), and its count of natural_t: a package variable, off any goroutine's
// stack, guarded by vmInfoMu.
var (
	vmInfoMu    sync.Mutex
	vmInfo      [taskVMInfoWords]uint64
	vmInfoCount uint32
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// footprint returns the process's physical footprint in bytes, the figure
// Xcode's memory gauge shows (task_vm_info.phys_footprint), 0 when the
// kernel does not tell it.
func footprint() int64 {
	vmInfoMu.Lock()
	defer vmInfoMu.Unlock()
	// The count is in natural_t, 32 bits.
	vmInfoCount = taskVMInfoWords * 2
	if av.taskInfo.Call(uintptr(av.taskSelf), taskVMInfo, uintptr(unsafe.Pointer(&vmInfo[0])), uintptr(unsafe.Pointer(&vmInfoCount))) != 0 { //nolint:gosec // G103: package variables, written during the call
		return 0
	}
	if vmInfoCount < (physFootprintAt+1)*2 {
		return 0
	}

	return int64(vmInfo[physFootprintAt]) //nolint:gosec // G115: a byte count far below 2^63
}
