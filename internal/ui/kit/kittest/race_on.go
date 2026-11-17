// Adapted from jplayer-mobile/internal/ui/views/viewstest/race_on.go at 49de3c2.

//go:build race

package kittest

// Race is true under the race detector, whose instrumentation makes every
// rendered frame several times dearer: Mount then reduces motion (the
// animations jump to their end), unless a test pins motion and says so.
const Race = true
