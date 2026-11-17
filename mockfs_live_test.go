//go:build live

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLiveBuildEmbedsNoMock: the live build compiles no mock folder in, so
// the app talks to the live API (internal/app/apimode.go).
func TestLiveBuildEmbedsNoMock(t *testing.T) {
	t.Parallel()

	assert.Nil(t, mockAPI())
}
