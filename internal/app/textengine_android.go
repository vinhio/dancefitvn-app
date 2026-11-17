//go:build android

package app

import (
	"log"

	"github.com/gdeskdev/desk/bridge/text"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// androidFontLists are the system's font lists, newest first: Android 15
// moved the fallback chain to font_fallback.xml, fonts.xml before.
var androidFontLists = []string{"/system/etc/font_fallback.xml", "/system/etc/fonts.xml"}

// androidFontDirs are the folders the lists' files are in.
var androidFontDirs = []string{"/system/fonts", "/product/fonts"}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// useAppTextEngine makes colorText, gotext with typeset for emoji, the
// text engine of the app's windows. Call it after RegisterFonts and
// before the first window opens.
func useAppTextEngine() error {
	if err := text.Replace(appTextEngine, func() (text.Engine, error) {
		return newAndroidText(androidFontLists, androidFontDirs, log.Printf)
	}); err != nil {
		return err
	}

	return text.SetDefault(appTextEngine)
}
