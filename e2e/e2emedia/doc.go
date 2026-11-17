// Package e2emedia serves the device tests' media on 127.0.0.1 (e2e/engine
// on the iPhone, e2e/tvengine on the Apple TV): the mock API's media folder
// through internal/api/mock's Server, as the app does, and a gate in front
// of it that plays the CDN's part for signed links (an expired link, or
// links revoked while the media plays, answer 403) and serves a long HLS
// playlist made of the mock's segments, so that a revoked link is met
// while the media plays rather than after it was all fetched. It holds no
// URL of its own: every address is the loopback listener's.
package e2emedia
