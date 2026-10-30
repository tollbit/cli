// Command gen-plugin writes the Claude plugin's generated files (see
// internal/generator/plugin) and prints each path it writes. Run it from the
// repo root: make plugin. It is a development tool, not part of the tollbit
// binary.
package main

import (
	"fmt"
	"os"

	"github.com/tollbit/cli/internal/generator/plugin"
)

func main() {
	if _, err := os.Stat("go.mod"); err != nil {
		fmt.Fprintln(os.Stderr, "gen-plugin: run from the repo root (make plugin)")
		os.Exit(1)
	}
	paths, err := plugin.Generate(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen-plugin:", err)
		os.Exit(1)
	}
	for _, p := range paths {
		fmt.Println(p)
	}
}
