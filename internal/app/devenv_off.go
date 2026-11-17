//go:build !dancefitvndev

// Adapted from jplayer-mobile/internal/app/devenv_off.go at 49de3c2.

package app

// devBuild is not set in the build that ships: the app reads none of the
// variables of a developer's script (DFV_STATE, DFV_SETTLE, DFV_OFFLINE),
// and the linker leaves their code out (devenv_on.go).
const devBuild = false
