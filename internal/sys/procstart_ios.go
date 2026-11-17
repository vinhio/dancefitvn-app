//go:build ios

package sys

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/dl"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// libSystemPath is the C library of iOS and tvOS, which exports sysctl.
const libSystemPath = "/usr/lib/libSystem.B.dylib"

// The name of kern.proc.pid.<pid> (sys/sysctl.h): CTL_KERN, KERN_PROC,
// KERN_PROC_PID.
const (
	ctlKern     = 1
	kernProc    = 14
	kernProcPID = 1
)

// kinfoProcMax holds a struct kinfo_proc (648 bytes on arm64), whose
// extern_proc begins with p_starttime, a struct timeval: tv_sec (8
// bytes) then tv_usec (4).
const kinfoProcMax = 1024

// procStart is the start of the process, read once. The name, the
// buffer and its length are package memory that sysctl writes into
// (never a goroutine's stack, foundation/ffi/dl's rule); only the once
// touches them.
var procStart struct {
	once sync.Once
	at   time.Time
	err  error
	mib  [4]int32
	info [kinfoProcMax]byte
	size uintptr
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// processStart returns the kernel's start time of the process, read
// once.
func processStart() (time.Time, error) {
	procStart.once.Do(func() { procStart.at, procStart.err = readProcStart() })

	return procStart.at, procStart.err
}

// readProcStart asks sysctl for the kinfo_proc of the process and reads
// its p_starttime.
func readProcStart() (time.Time, error) {
	const op = "process start"
	lib, err := dl.Open(libSystemPath)
	if err != nil {
		return time.Time{}, fmt.Errorf("sys: %s: %w", op, err)
	}
	var sysctl dl.Fn
	if err := lib.Bind(dl.Sym{Fn: &sysctl, Name: "sysctl"}); err != nil {
		return time.Time{}, fmt.Errorf("sys: %s: %w", op, err)
	}
	p := &procStart
	p.mib = [4]int32{ctlKern, kernProc, kernProcPID, int32(os.Getpid())} //nolint:gosec // G115: a pid fits pid_t
	p.size = uintptr(len(p.info))
	r := sysctl.Call(uintptr(unsafe.Pointer(&p.mib[0])), uintptr(len(p.mib)), //nolint:gosec // G103: package memory, sysctl's out-parameters
		uintptr(unsafe.Pointer(&p.info[0])), uintptr(unsafe.Pointer(&p.size)), 0, 0) //nolint:gosec // G103: as above
	if int32(r) != 0 { //nolint:gosec // G115: sysctl answers an int
		return time.Time{}, fmt.Errorf("sys: %s: sysctl failed: %w", op, core.ErrUnsupported)
	}
	if p.size < 16 {
		return time.Time{}, fmt.Errorf("sys: %s: kinfo_proc of %d bytes: %w", op, p.size, core.ErrInvalid)
	}
	sec := int64(binary.LittleEndian.Uint64(p.info[0:8]))          //nolint:gosec // G115: time_t
	usec := int64(int32(binary.LittleEndian.Uint32(p.info[8:12]))) //nolint:gosec // G115: suseconds_t
	if sec <= 0 {
		return time.Time{}, fmt.Errorf("sys: %s: start time %d: %w", op, sec, core.ErrInvalid)
	}

	return time.Unix(sec, usec*int64(time.Microsecond)), nil
}
