//go:build live

package main

import "io/fs"

// mockAPI returns nil in a live build: the mock folder is not compiled in
// (mockfs.go), and the app talks to the real API.
func mockAPI() fs.FS { return nil }
