// Command fakecli is the program selfinstalltest builds for its fake CLI
// binaries and fake npm; see package fakecli.
package main

import (
	"os"

	"github.com/tollbit/cli/internal/selfinstall/selfinstalltest/fakecli"
)

func main() {
	os.Exit(fakecli.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
