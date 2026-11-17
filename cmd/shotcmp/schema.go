// Command shotcmp compares the layout.json and the screenshot of an app
// state with the prototype's (docs/plan/07-testing.md section 3.3).
//
// Adapted from jplayer-mobile/cmd/shotcmp (picture helpers: scale detection,
// area resampling, corner mask, side-by-side picture; the match of elements
// by name and index). Extended with the style comparison, the Android
// stretch rule, the per-element crop diff, rectangle masks, a JSON report
// and the DanceFITVN layout schema.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Layout is the content of a layout.json, written by the prototype capture
// (local/tools/capture_kit.py) and by the app's layout dump
// (internal/ui/kit/layoutdump). Units are CSS px on the prototype and
// points (DIP) on the app; both are the same number at scale 1.
type Layout struct {
	ID       string    `json:"id"`
	Platform string    `json:"platform"` // "mobile" or "tv"; empty: from the -platform flag
	Unit     string    `json:"unit"`
	Screen   Screen    `json:"screen"`
	Elements []Element `json:"elements"`
}

// Screen is the size of the captured root (#m, #tv or the window).
type Screen struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Element is one recorded box. Elements match by (Name, Index); Name is the
// widget's key (data-act/go/fid key, else tag.class.class) and Index the
// 1-based occurrence of the name in the file. X, Y are relative to the root.
type Element struct {
	Key    string  `json:"key"`    // Name, or Name#Index when Index > 1
	Name   string  `json:"name"`   // key without the #n suffix
	Index  int     `json:"index"`  // 1-based occurrence; 0 counts as 1
	Path   string  `json:"path"`   // informational
	Parent string  `json:"parent"` // informational
	Tag    string  `json:"tag"`    // informational
	Text   string  `json:"text"`   // visible text, at most 80 characters
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	W      float64 `json:"w"`
	H      float64 `json:"h"`
	Style  Style   `json:"style"`
}

// Style is the computed style of an element, every value a CSS string
// ("14px", "rgb(255, 208, 40)", "rgba(0, 0, 0, 0)", "700"). An empty field
// means "not recorded" and is not compared. Only the fields below are
// compared; the prototype records more (padding, margin, ...), which the app
// may leave out.
type Style struct {
	FontFamily    string   `json:"fontFamily"`
	FontSize      string   `json:"fontSize"`
	FontWeight    string   `json:"fontWeight"`
	LineHeight    string   `json:"lineHeight"`
	LetterSpacing string   `json:"letterSpacing"`
	Color         string   `json:"color"`
	Background    string   `json:"background"`
	Radius        string   `json:"radius"` // top-left radius
	Border        string   `json:"border"` // top width + color: "1px rgb(1, 2, 3)"
	Opacity       string   `json:"opacity"`
	Radii         []string `json:"radii,omitempty"`
}

// match returns the (name, index) key of the element.
func (e Element) match() string {
	i := e.Index
	if i < 1 {
		i = 1
	}
	return fmt.Sprintf("%s#%d", e.Name, i)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// readLayout loads and checks a layout.json.
func readLayout(path string) (Layout, error) {
	var l Layout
	b, err := os.ReadFile(path)
	if err != nil {
		return l, err
	}
	if err := json.Unmarshal(b, &l); err != nil {
		return l, fmt.Errorf("%s: %w", path, err)
	}
	if l.Screen.W <= 0 || l.Screen.H <= 0 {
		return l, fmt.Errorf("%s: screen size missing", path)
	}
	for i := range l.Elements {
		e := &l.Elements[i]
		if e.Name == "" {
			e.Name = e.Key
		}
		if e.Index < 1 {
			e.Index = 1
		}
	}
	return l, nil
}
