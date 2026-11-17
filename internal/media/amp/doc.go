// Adapted from jplayer-mobile/internal/media/amp/doc.go at 49de3c2.

// Package amp is the Android playback engine on android.media.MediaPlayer,
// reached through JNI. While a link plays, or prepares to, the engine holds
// a Wi-Fi lock (WIFI_MODE_FULL_HIGH_PERF), so a stream with the screen off
// does not stall on a sleeping Wi-Fi; from a media's end to the next's
// start it keeps the CPU awake (a hand-off wake lock), so a queue goes on
// with the screen off. It builds only with the android tag.
package amp
