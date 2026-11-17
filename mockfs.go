//go:build !live

package main

import (
	"embed"
	"io/fs"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// mockFiles is the mock API of mock/dancefitvn-mock-api, compiled into the
// app (docs/plan/05-data.md §4). go:embed cannot reach a parent folder, so
// the root package, which is one, embeds it, and mock/ stays untouched.
// A directory pattern skips names that start with "_", so the _sample
// media are listed. A build with -tags live leaves all of it out.
//
//go:embed mock/dancefitvn-mock-api/routes.json
//go:embed mock/dancefitvn-mock-api/README.md
//go:embed mock/dancefitvn-mock-api/*/*.json
//go:embed mock/dancefitvn-mock-api/media
//go:embed all:mock/dancefitvn-mock-api/media/hls/_sample
//go:embed mock/dancefitvn-mock-api/media/audio/_sample.mp3
var mockFiles embed.FS

// mockRoot is the mock folder inside mockFiles.
const mockRoot = "mock/dancefitvn-mock-api"

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// mockAPI returns the embedded mock folder as the root of an fs.FS
// (routes.json, errors/, media/ ...), for mock.New.
func mockAPI() fs.FS {
	sub, err := fs.Sub(mockFiles, mockRoot)
	if err != nil { // the root is a constant that exists: not reachable
		return mockFiles
	}

	return sub
}
