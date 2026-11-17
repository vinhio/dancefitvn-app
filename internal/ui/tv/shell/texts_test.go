//go:build tvos

package shell_test

import "dancefitvn-app/locales"

// texts returns the app's strings.
func texts() map[string]string { return locales.MustTexts() }
