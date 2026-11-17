//go:build dancefitvndev

// Adapted from jplayer-mobile/internal/app/devenv_on.go at 49de3c2.

package app

// devBuild is set in a developer's build, made with the dancefitvndev tag
// (scripts/dev-build.sh, gdesk build -tags dancefitvndev): the app then
// reads the variables of the shot script (docs/plan/07-testing.md §3.3):
// DFV_STATE, DFV_SETTLE and DFV_OFFLINE, from the environment on iOS
// (SIMCTL_CHILD_ of simctl launch) and from the launch intent's extras on
// Android (am start --es DFV_STATE M02, devenv_android.go).
const devBuild = true
