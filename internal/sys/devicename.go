package sys

import (
	"strings"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// fallbackDeviceName is the name sent when the system tells none.
const fallbackDeviceName = "Apple TV"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// DeviceName returns the name of the device the user gave it
// (UIDevice.name: "Apple TV phòng khách"; on tvOS 16 and later the system
// may answer only the model, "Apple TV"), sent as the device's name when a
// TV asks for a pairing code (Q-P7-13). Where the system tells none
// (outside iOS and tvOS, or UIKit unreachable) it is "Apple TV". Call it on
// the UI thread.
func DeviceName() string {
	name, err := deviceName()
	name = strings.TrimSpace(name)
	if err != nil || name == "" {
		return fallbackDeviceName
	}

	return name
}
