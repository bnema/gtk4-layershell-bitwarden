package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bnema/gtk4-layershell-bitwarden/internal/adapters/cli/cobra"
)

var version = "dev"

func main() {
	root := cobra.NewRootCommand(cobra.Options{Version: version})
	if err := root.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
