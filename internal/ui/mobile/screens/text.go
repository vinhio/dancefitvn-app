//go:build !tvos

package screens

import "dancefitvn-app/internal/ui/mobile/env"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Fill replaces each "{name}" of a locale template by the value after name in kv
// (name, value, name, value ...); it is env.Fill.
func Fill(tpl string, kv ...string) string { return env.Fill(tpl, kv...) }
