// Adapted from jplayer-tv/internal/ui/theme/faces.go at 781bee5.

package theme

import (
	"math"
	"runtime"

	"github.com/gdeskdev/desk/bridge/text"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Role is what a piece of text is for: which of the prototype's three
// font variables (--disp, --body, --mono, index.html:11) it is set in.
type Role uint8

// The roles.
const (
	RoleUnknown Role = iota
	// RoleDisplay is --disp, Barlow Condensed: the brand, headings,
	// numbers.
	RoleDisplay
	// RoleBody is --body, Inter: everything not set in the other two. It
	// is what RoleUnknown reads as.
	RoleBody
	// RoleMono is --mono, JetBrains Mono: times and codes.
	RoleMono
)

// Naming is how a text engine is asked for a face.
type Naming uint8

// The ways of naming a face (jplayer-mobile internal/ui/theme/faces.go,
// PLAN D5).
const (
	NamingUnknown Naming = iota
	// NamingCoreText asks for the face's PostScript name at
	// text.WeightRegular: gDesk's Core Text engine opens a named family's
	// regular face and maps every weight from 600 up to the bold trait
	// (bridge/text/coretext/engine.go font; GT15 of jPlayer TV), so Inter
	// 600, 700 and 800 would all draw as Inter Bold. Each static face is
	// named whole instead. Used on iOS, tvOS, and the Mac in tests.
	NamingCoreText
	// NamingGoText asks for the typographic family (name ID 16) and the
	// weight: gotext knows no PostScript name, falls back on the first
	// family registered for one it does not know, and picks the nearest
	// weight of a family itself. Used on Android.
	NamingGoText
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// faceSpec is one static face of assets/fonts.
type faceSpec struct {
	// PostScript is the face's PostScript name (name ID 6), by which Core
	// Text finds exactly this file; the file is fonts/<PostScript>.ttf.
	PostScript string
	// Weight is the face's weight class (OS/2 usWeightClass), by which
	// gotext picks it within its family.
	Weight text.Weight
}

// familySpec is the family a role is set in.
type familySpec struct {
	// family is the typographic family gotext is asked for.
	family string
	// loaded are the weights the prototype loads (index.html:8): CSS
	// matches the weight a rule asks for against these (CSS Fonts 4,
	// section 5.2).
	loaded []text.Weight
	// faces are the static faces shipped, by weight.
	faces []faceSpec
}

// families are the families of the roles, in the order of the files in
// assets/fonts (PLAN D5). Barlow Condensed 600 is loaded by the prototype
// but no rule draws it (Q-P0-7): 700, the nearest shipped weight, stands
// in for it.
var families = [...]familySpec{
	RoleDisplay: {
		family: "Barlow Condensed",
		loaded: []text.Weight{600, 700, 800},
		faces: []faceSpec{
			{"BarlowCondensed-Bold", text.WeightBold},
			{"BarlowCondensed-ExtraBold", text.WeightExtraBold},
		},
	},
	RoleBody: {
		family: "Inter",
		loaded: []text.Weight{400, 500, 600, 700, 800},
		faces: []faceSpec{
			{"Inter-Regular", text.WeightRegular},
			{"Inter-Medium", text.WeightMedium},
			{"Inter-SemiBold", text.WeightSemibold},
			{"Inter-Bold", text.WeightBold},
			{"Inter-ExtraBold", text.WeightExtraBold},
		},
	},
	RoleMono: {
		family: "JetBrains Mono",
		loaded: []text.Weight{500, 700},
		faces: []faceSpec{
			{"JetBrainsMono-Medium", text.WeightMedium},
			{"JetBrainsMono-Bold", text.WeightBold},
		},
	},
}

// Faces names the app's faces for one text engine.
type Faces struct {
	naming Naming
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// DefaultNaming returns the naming of the text engine this program draws
// with (bridge/text/auto): Core Text on iOS, tvOS and macOS, gotext
// elsewhere (Android).
func DefaultNaming() Naming {
	switch runtime.GOOS {
	case "ios", "darwin":
		return NamingCoreText
	default:
		return NamingGoText
	}
}

// NewFaces returns the faces named for n; NamingUnknown, and any naming
// out of range, reads as DefaultNaming.
func NewFaces(n Naming) Faces {
	if n != NamingCoreText && n != NamingGoText {
		n = DefaultNaming()
	}

	return Faces{naming: n}
}

// Naming returns how f names faces; the zero Faces names them for
// DefaultNaming.
func (f Faces) Naming() Naming {
	if f.naming == NamingUnknown {
		return DefaultNaming()
	}

	return f.naming
}

// Face returns the face CSS would draw role at size and weight in: the
// weight matched against the weights the prototype loads (CSS Fonts 4,
// section 5.2), then the shipped face nearest to it, named for the
// engine: on Core Text its PostScript name at text.WeightRegular, on
// gotext its family at its own weight. A zero weight reads as 400; the
// size is passed on as it is. It does not allocate.
func (f Faces) Face(r Role, size float32, weight text.Weight) text.Face {
	if weight == 0 {
		weight = text.WeightRegular
	}
	fam := &families[familyIndex(r)]
	spec := pickFace(fam, weight)
	if f.Naming() == NamingCoreText {
		return text.Face{Family: spec.PostScript, Size: size, Weight: text.WeightRegular}
	}

	return text.Face{Family: fam.family, Size: size, Weight: spec.Weight}
}

// Family returns the typographic family role is set in: "Barlow
// Condensed", "Inter" or "JetBrains Mono".
func Family(r Role) string { return families[familyIndex(r)].family }

// FaceNames returns the PostScript names of every face Face can return,
// in the order of the files in assets/fonts: each is the stem of its file.
func FaceNames() []string {
	var names []string
	for _, fam := range families {
		for _, s := range fam.faces {
			names = append(names, s.PostScript)
		}
	}

	return names
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// familyIndex returns the index in families of role r: RoleUnknown, and
// any role out of range, read as RoleBody.
func familyIndex(r Role) Role {
	switch r {
	case RoleDisplay, RoleMono, RoleBody:
		return r
	case RoleUnknown:
		return RoleBody
	default:
		return RoleBody
	}
}

// pickFace returns the face of fam CSS would draw at weight: the weight
// matched against the loaded ones, then the shipped face nearest to it
// (the first on a tie).
func pickFace(fam *familySpec, weight text.Weight) faceSpec {
	want := matchWeight(fam.loaded, weight)
	best := fam.faces[0]
	for _, s := range fam.faces[1:] {
		if absDiff(s.Weight, want) < absDiff(best.Weight, want) {
			best = s
		}
	}

	return best
}

// matchWeight returns the weight CSS Fonts section 5.2 picks among have for
// the desired weight: between 400 and 500 the nearest above up to 500, then
// below, then above 500; under 400 the nearest below, then above; over 500
// the nearest above, then below. It returns want when have is empty.
func matchWeight(have []text.Weight, want text.Weight) text.Weight {
	const top = text.Weight(math.MaxUint16)
	switch {
	case want >= 400 && want <= 500:
		if w, ok := nearestWeight(have, want, 500, true); ok {
			return w
		}
		if w, ok := nearestWeight(have, 0, want, false); ok {
			return w
		}
		if w, ok := nearestWeight(have, 500, top, true); ok {
			return w
		}
	case want < 400:
		if w, ok := nearestWeight(have, 0, want, false); ok {
			return w
		}
		if w, ok := nearestWeight(have, want, top, true); ok {
			return w
		}
	default:
		if w, ok := nearestWeight(have, want, top, true); ok {
			return w
		}
		if w, ok := nearestWeight(have, 0, want, false); ok {
			return w
		}
	}

	return want
}

// nearestWeight returns the weight of have within [lo, hi] that is the
// lowest (up) or the highest (not up), and whether there is one.
func nearestWeight(have []text.Weight, lo, hi text.Weight, up bool) (text.Weight, bool) {
	best, found := text.Weight(0), false
	for _, w := range have {
		if w < lo || w > hi {
			continue
		}
		if !found || (up && w < best) || (!up && w > best) {
			best, found = w, true
		}
	}

	return best, found
}

// absDiff returns |a - b|.
func absDiff(a, b text.Weight) text.Weight {
	if a > b {
		return a - b
	}

	return b - a
}
