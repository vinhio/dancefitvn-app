// Adapted from jplayer-mobile/internal/media/enginekit/doc.go at 49de3c2.

// Package enginekit is the engines' logic that does not depend on the system: the table of errors by phase, the state machine of Android's MediaPlayer, CMTime, the 250 ms tick, the count of the seeks under way, the size limits, the probe's time and picture size, the rotation, and a reference YUV to RGBA. The engines in avf and amp only call the system; this part is tested on the Mac.
package enginekit
