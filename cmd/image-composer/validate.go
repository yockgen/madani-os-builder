package main

import (
	"fmt"
	"os"

	utils "github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/validate"
	"github.com/spf13/cobra"
)

// createValidateCommand creates the validate subcommand
func createValidateCommand() *cobra.Command {
	validateCmd := &cobra.Command{
		Use:               "validate SPEC_FILE",
		Short:             "Validate a spec file against the schema",
		Long:              `Validate that the given JSON spec file conforms to the schema.`,
		Args:              cobra.ExactArgs(1),
		RunE:              executeValidate,
		ValidArgsFunction: jsonFileCompletion,
	}

	return validateCmd
}

// executeValidate handles the validate command execution logic
func executeValidate(cmd *cobra.Command, args []string) error {
	logger := utils.Logger()

	// Check if spec file is provided as first positional argument
	if len(args) < 1 {
		return fmt.Errorf("no spec file provided, usage: image-composer validate SPEC_FILE")
	}
	specFile := args[0]

	logger.Infof("Validating spec file: %s", specFile)

	// Read the file
	data, err := os.ReadFile(specFile)
	if err != nil {
		return fmt.Errorf("reading spec file: %v", err)
	}

	// Validate the JSON against schema
	if err := validate.ValidateComposerJSON(data); err != nil {
		return fmt.Errorf("validation failed: %v", err)
	}

	logger.Info("Spec file is valid")
	return nil
}
