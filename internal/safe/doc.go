// Package safe is the app's guard against a panic on a goroutine nobody
// recovers: a decoder meeting a broken picture, a worker meeting a broken
// file. A guarded call turns the panic into an error and logs it; the app
// degrades (a placeholder, a failed download) instead of ending.
//
// gDesk recovers the panics of the functions it runs for the app (queued UI
// work, ui.Go's work and then, image loads) and hands them to one handler:
// Install makes HandlePanic that handler, so that they are logged, and
// treated in tests, as the guards' are.
package safe
