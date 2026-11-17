// Adapted from jplayer-mobile/cmd/brandcheck/main.go at 49de3c2.

// Command brandcheck verifies the pictures docs/brand/generate.sh makes for
// DanceFITVN (docs/brand/BRAND.md): the sizes and the alpha rules of every
// field of gdesk.json that takes an image, the launcher layers' safe zones,
// the tvOS brand assets (roles, sizes, scales, layers, opaque back layer),
// gdesk.json's wiring to those files, and docs/brand/source/SHA256SUMS. With
// -shots it also verifies the store shots of a folder (size, no alpha).
//
//	go run ./cmd/brandcheck [-root DIR] [-shots DIR]
//
// It exits 1 when a check fails, naming each problem.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main with its streams and arguments, returning the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("brandcheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "the repository root")
	shots := fs.String("shots", "", "a folder of store shots to check as well")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := checkRoot(*root); err != nil {
		report(stderr, "brandcheck:", err)
		return 2
	}
	problems := Check(*root)
	if *shots != "" {
		problems = append(problems, CheckShots(*shots)...)
	}
	for _, p := range problems {
		report(stderr, "brandcheck:", p)
	}
	if len(problems) > 0 {
		report(stderr, "brandcheck:", len(problems), "problem(s)")
		return 1
	}
	report(stdout, "brandcheck: ok")

	return 0
}

// report writes a line to w; a failed write of a diagnostic has nowhere to go.
func report(w io.Writer, a ...any) {
	_, _ = fmt.Fprintln(w, a...)
}
