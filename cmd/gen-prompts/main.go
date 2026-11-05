// Command gen-prompts writes the generated prompts in prompts/ (see
// internal/generator/prompts) and prints each path it writes. Run it from the
// repo root: make prompts. It is a development tool, not part of the tollbit
// binary.
package main

import (
	"fmt"
	"os"

	"github.com/tollbit/cli/internal/generator/prompts"
)

func main() {
	if _, err := os.Stat("go.mod"); err != nil {
		fmt.Fprintln(os.Stderr, "gen-prompts: run from the repo root (make prompts)")
		os.Exit(1)
	}
	paths, err := prompts.Generate(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen-prompts:", err)
		os.Exit(1)
	}
	for _, p := range paths {
		fmt.Println(p)
	}
}
