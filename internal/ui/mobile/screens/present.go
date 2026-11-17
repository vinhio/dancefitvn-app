//go:build !tvos

// Adapted from dancefitvn-app/internal/ui/mobile/kitdemo/build.go (category,
// chipKind, shortName, current: the view-model helpers of the course screens,
// copied and not imported, Q-P3-25).

package screens

import (
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/poster"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// CardCategory maps a course's category to the course card's.
func CardCategory(c model.Category) kit.CardCategory {
	switch c {
	case model.CategorySpecial:
		return kit.CardSpecial
	case model.CategoryIntro:
		return kit.CardIntro
	case model.CategoryRegular, model.CategoryUnknown:
		return kit.CardRegular
	default:
		return kit.CardRegular
	}
}

// ChipKind maps a course's category to the chip's.
func ChipKind(c model.Category) kit.ChipKind {
	switch c {
	case model.CategorySpecial:
		return kit.ChipDb
	case model.CategoryIntro:
		return kit.ChipGt
	case model.CategoryRegular, model.CategoryUnknown:
		return kit.ChipTh
	default:
		return kit.ChipTh
	}
}

// ShortName is a course's name without the brand, "Set 35" (the prototype's
// `short.replace('SET','Set')`).
func ShortName(c model.Course) string { return c.ShortName() }

// IsCurrent tells whether l is the lesson under way: some of it seen, not all.
func IsCurrent(l model.Lesson) bool { return l.Progress.PositionSec > 0 && !l.Progress.Watched }

// CoursePoster returns the 156 x 100 cover of a course in a strip
// (`poster(id,156,100,{ts:20})`).
func CoursePoster(e *env.Env, c model.Course) *poster.Poster {
	s := poster.ForCourse(c)
	s.TitleSize = 20
	p := poster.New(s, 156, 100)
	p.SetCover(e.Cover, c.CoverURL) // the real cover once loaded (K14)

	return p
}
