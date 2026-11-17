package theme

import "github.com/gdeskdev/desk/foundation/core/color"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Colors are the literal colors the prototype's CSS writes outside :root
// (02 section 2): each field names the selector and line of
// reference/prototype/index.html it comes from. theme_test.go compares every
// one with the CSS.
type Colors struct {
	// TabBar is `.tabs{background:#1A1B1F}` (:79).
	TabBar color.Color
	// ToastBg and ToastText are `.toast{background:#F4F4F2;color:#141518}` (:131).
	ToastBg, ToastText color.Color
	// ChipRegularBg and ChipRegularText are `.chip.c-th` (:51).
	ChipRegularBg, ChipRegularText color.Color
	// ChipSpecialBg is `.chip.c-db{background:rgba(77,163,255,.18)}` (:52).
	ChipSpecialBg color.Color
	// ChipIntroBg is `.chip.c-gt` and `.chip.okc` (:53, :56).
	ChipIntroBg color.Color
	// ChipDarkBg is `.chip.k{background:rgba(0,0,0,.55)}` (:55).
	ChipDarkBg color.Color
	// PillOn is `.tabs button.on .pill` (:82).
	PillOn color.Color
	// LessonCur is `.les.cur{background:rgba(255,208,40,.12)}` (:93).
	LessonCur color.Color
	// OfflineBg is `.offbar{background:rgba(255,176,32,.16)}` (:132).
	OfflineBg color.Color
	// RoundDark is `.rb.dk{background:rgba(0,0,0,.4)}` (:85).
	RoundDark color.Color
	// CropBg is `.crop{background:#2A2D33}` (:167).
	CropBg color.Color
	// RingWhite13 is `.poster .ring{border:solid rgba(255,255,255,.13)}` (:66).
	RingWhite13 color.Color
	// CatSpecial, CatIntro and CatRegular are the category colors of a course
	// card (courseCardH, :371): var(--info), var(--ok) and #FF6B6E.
	CatSpecial, CatIntro, CatRegular color.Color
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// literals are the Colors of the app.
var literals = Colors{
	TabBar:          color.RGB(0x1a, 0x1b, 0x1f),
	ToastBg:         color.RGB(0xf4, 0xf4, 0xf2),
	ToastText:       color.RGB(0x14, 0x15, 0x18),
	ChipRegularBg:   color.RGBA(229, 56, 59, .18),
	ChipRegularText: color.RGB(0xff, 0x6b, 0x6e),
	ChipSpecialBg:   color.RGBA(77, 163, 255, .18),
	ChipIntroBg:     color.RGBA(61, 214, 140, .16),
	ChipDarkBg:      color.RGBA(0, 0, 0, .55),
	PillOn:          color.RGBA(255, 208, 40, .16),
	LessonCur:       color.RGBA(255, 208, 40, .12),
	OfflineBg:       color.RGBA(255, 176, 32, .16),
	RoundDark:       color.RGBA(0, 0, 0, .4),
	CropBg:          color.RGB(0x2a, 0x2d, 0x33),
	RingWhite13:     color.RGBA(255, 255, 255, .13),
	CatSpecial:      color.RGB(0x4d, 0xa3, 0xff),
	CatIntro:        color.RGB(0x3d, 0xd6, 0x8c),
	CatRegular:      color.RGB(0xff, 0x6b, 0x6e),
}
