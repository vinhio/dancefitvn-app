//go:build !android

// Adapted from jplayer-mobile/internal/app/env_other.go at 49de3c2.

package app

// applyLaunchEnv does nothing where the environment reaches the program:
// the iOS simulator passes the SIMCTL_CHILD_ variables of simctl launch,
// the desktop its own.
func applyLaunchEnv() {}
