package main

import (
	"fmt"
	"os"

	"github.com/xonecas/mysis/internal/cli"
	"github.com/xonecas/mysis/internal/styles"
)

// Version is set at build time via ldflags.
var Version = "dev"

func main() {
	if err := cli.Run(Version); err != nil {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Error: "+err.Error()))
		os.Exit(1)
	}
}
