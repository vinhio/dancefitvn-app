//go:build !tvos

package screens

import (
	"time"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/mobile/env"
)

// NewHomeContentForTest returns the content of Home for an answer built by the test.
func NewHomeContentForTest(e *env.Env, h model.Home) *homeContent {
	return newHomeContent(e, h, time.Date(2025, 11, 15, 9, 0, 0, 0, time.UTC))
}
