package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateValidateCommand_HasMergedFlag(t *testing.T) {
	cmd := createValidateCommand()
	flag := cmd.Flags().Lookup("merged")
	if flag == nil {
		t.Fatalf("--merged flag not found on validate command")
	}
}

func TestValidateCommand_MissingTemplateArg(t *testing.T) {
	cmd := createValidateCommand()
	// no args should yield an error (template file required)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error when template argument is missing")
	}
}

func TestValidateCommand_WithEmptyTemplateFile(t *testing.T) {
	tmp := t.TempDir()
	tmpl := filepath.Join(tmp, "tmpl.yaml")
	if err := os.WriteFile(tmpl, []byte(""), 0644); err != nil {
		t.Fatalf("write temp template: %v", err)
	}

	cmd := createValidateCommand()
	cmd.SetArgs([]string{tmpl, "--merged"})
	// We don't assert success/failure semantics (depends on parser),
	// but we assert the command runs without panicking.
	_ = cmd.Execute()
}
