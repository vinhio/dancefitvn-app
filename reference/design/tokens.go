// Code generated from tokens.json — DO NOT EDIT.
// Night Stage design tokens for DanceFITVN (gDesk Mobile + Apple TV).
package tokens

// Color is 0xRRGGBBAA.
type Color uint32

func Hex(v uint32) Color { return Color(v) }
func RGBA(r, g, b, a uint8) Color {
	return Color(uint32(r)<<24 | uint32(g)<<16 | uint32(b)<<8 | uint32(a))
}

type Palette struct {
	Bg         Color
	Surface    Color
	Surface2   Color
	Border     Color
	Text       Color
	Text2      Color
	Text3      Color
	Primary    Color
	OnPrimary  Color
	AccentText Color
	Red        Color
	Info       Color
	Success    Color
	Warning    Color
	Danger     Color
	Scrim      Color
}

var Dark = Palette{
	Bg:         Hex(0x141518FF),
	Surface:    Hex(0x1F2125FF),
	Surface2:   Hex(0x2A2D33FF),
	Border:     Hex(0x33363DFF),
	Text:       Hex(0xF4F4F2FF),
	Text2:      Hex(0x8E9196FF),
	Text3:      Hex(0x5E6167FF),
	Primary:    Hex(0xFFD028FF),
	OnPrimary:  Hex(0x141518FF),
	AccentText: Hex(0xFFD028FF),
	Red:        Hex(0xE5383BFF),
	Info:       Hex(0x4DA3FFFF),
	Success:    Hex(0x3DD68CFF),
	Warning:    Hex(0xFFB020FF),
	Danger:     Hex(0xFF5A5FFF),
	Scrim:      RGBA(0, 0, 0, 140),
}

var Light = Palette{
	Bg:         Hex(0xF4F3EEFF),
	Surface:    Hex(0xFFFFFFFF),
	Surface2:   Hex(0xECEAE3FF),
	Border:     Hex(0xE0DED6FF),
	Text:       Hex(0x141518FF),
	Text2:      Hex(0x6A6D73FF),
	Text3:      Hex(0xA3A5AAFF),
	Primary:    Hex(0xFFD028FF),
	OnPrimary:  Hex(0x141518FF),
	AccentText: Hex(0x9A7300FF),
	Red:        Hex(0xD62828FF),
	Info:       Hex(0x1F6FD1FF),
	Success:    Hex(0x1E9E5AFF),
	Warning:    Hex(0xC77700FF),
	Danger:     Hex(0xD93636FF),
	Scrim:      RGBA(0, 0, 0, 115),
}

const (
	FontDisplay = "Barlow Condensed"
	FontBody    = "Inter"
	FontMono    = "JetBrains Mono"
)

const (
	SpaceXs         = 4
	SpaceSm         = 8
	SpaceMd         = 12
	SpaceLg         = 16
	SpaceXl         = 20
	SpaceXXL        = 24
	SpaceXXXL       = 32
	RadiusSm        = 10
	RadiusMd        = 14
	RadiusLg        = 16
	RadiusXl        = 20
	RadiusPill      = 999
	HeightButton    = 54
	HeightInput     = 54
	HeightLessonRow = 58
	HeightTabBar    = 84
	HeightTvButton  = 88
	Grid            = 4
	MobileMargin    = 20
	TvSafeH         = 90
	TvSafeV         = 60
	MotionFastMs    = 150
	MotionBaseMs    = 250
	TVFocusScale    = 1.06
)

// TypeStyle: size in pt for mobile and TV (min/max of the range).
type TypeStyle struct {
	Font       string
	Mobile, TV [2]float32
	Weight     [2]int
	Caps       bool
	Tracking   float32
}

var Type = map[string]TypeStyle{
	"display": {Font: FontDisplay, Mobile: [2]float32{30, 36}, TV: [2]float32{84, 110}, Weight: [2]int{700, 800}, Caps: true, Tracking: 0},
	"title":   {Font: FontDisplay, Mobile: [2]float32{22, 26}, TV: [2]float32{44, 60}, Weight: [2]int{700, 700}, Caps: true, Tracking: 0},
	"section": {Font: FontDisplay, Mobile: [2]float32{18, 18}, TV: [2]float32{36, 36}, Weight: [2]int{700, 700}, Caps: true, Tracking: 1},
	"body":    {Font: FontBody, Mobile: [2]float32{14, 15}, TV: [2]float32{28, 30}, Weight: [2]int{400, 600}, Caps: false, Tracking: 0},
	"caption": {Font: FontBody, Mobile: [2]float32{12, 12}, TV: [2]float32{22, 24}, Weight: [2]int{500, 500}, Caps: false, Tracking: 0},
	"eyebrow": {Font: FontBody, Mobile: [2]float32{10, 11}, TV: [2]float32{22, 24}, Weight: [2]int{700, 700}, Caps: true, Tracking: 1.3},
	"mono":    {Font: FontMono, Mobile: [2]float32{11, 12}, TV: [2]float32{26, 30}, Weight: [2]int{500, 700}, Caps: false, Tracking: 0},
}
