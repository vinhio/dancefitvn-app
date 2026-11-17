// Command main writes the Go source of package tokens from the design tokens
// file: go run ./gen/cmd <tokens.json> <out.go>, from internal/tokens.
package main

import (
	"fmt"
	"os"

	"dancefitvn-app/internal/tokens/gen"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: gen <tokens.json> <out.go>")
		os.Exit(2)
	}
	in, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
	out, err := gen.Generate(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(os.Args[2], out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}
