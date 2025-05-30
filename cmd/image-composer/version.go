package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// createVersionCommand creates the version subcommand
func createVersionCommand() *cobra.Command {
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Run:   executeVersion,
	}

	return versionCmd
}

// executeVersion handles the version command logic
func executeVersion(cmd *cobra.Command, args []string) {
	fmt.Printf("Image Composer Tool v%s\n", Version)
	fmt.Printf("Build Date: %s\n", BuildDate)
	fmt.Printf("Commit: %s\n", CommitSHA)
}
