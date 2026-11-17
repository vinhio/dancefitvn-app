//go:build !tvos

// Package kitdemo draws one page per prototype state out of the kit's widgets
// alone, each component at the box the prototype's layout.json gives it and
// everything else left as --bg, so that the kit can be compared with the
// prototype component by component (cmd/shotcmp) before any screen exists
// (docs/plan/01 section 4, local/plans/P2.md P2.7).
//
// A page is a Page: the prototype's state id ("M03", "M07"...), and a
// function that builds its widget from the mock data. Gallery lists them and
// opens one; the app's -tags kitdemo build mounts it (internal/app).
// The shots test renders every page on Core Text (iOS) and gotext (Android)
// and writes the picture and layout.json of each with internal/ui/kit/layoutdump.
package kitdemo
