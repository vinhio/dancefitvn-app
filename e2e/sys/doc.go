// Adapted from jplayer-mobile/e2e/sys/doc.go at 49de3c2.

// Package sys holds the device tests of the system services of
// internal/sys (local/plans/P5.md, P5.1): test files only, run inside a
// real gDesk app on the iOS simulator (e2e/devtest-ios.sh ./e2e/sys) or
// the Android emulator (e2e/devtest-android.sh ./e2e/sys). They check the
// network's kind (on the emulator Wi-Fi, mobile data and airplane mode
// turned off and on through the script's driver; the simulator has the
// Mac's network and no mobile data), the free space of the audio folder
// against statfs, and the audio folder itself (made, out of backups, the
// folder of before moved in). On the Mac the tests skip. See
// e2e/README.md.
package sys
