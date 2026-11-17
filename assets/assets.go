// Package assets holds the files embedded in the app: the fonts.
package assets

import "embed"

// Fonts holds fonts/*.ttf, the nine static faces the app draws in (PLAN
// D5: Barlow Condensed 700 and 800, Inter 400 to 800, JetBrains Mono 500
// and 700, each subset to Latin and Vietnamese), and fonts/*.txt: their
// SIL Open Font License texts, which travel with the fonts, and
// SHA256SUMS.txt, which internal/app/fonts.go names the cached copies by.
// fonts/README.md says where each file comes from and how
// scripts/fonts/ makes it.
//
//go:embed fonts/*.ttf fonts/*.txt
var Fonts embed.FS
