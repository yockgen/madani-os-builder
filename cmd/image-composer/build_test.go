package main

import (
	"testing"
)

func TestExecuteBuild_NoTemplateArg(t *testing.T) {
	cmd := createBuildCommand()
	// No args -> should error
	err := executeBuild(cmd, []string{})
	if err == nil {
		t.Fatalf("expected error when no template file is provided")
	}
}

func TestInitProvider_Unsupported(t *testing.T) {
	if _, err := InitProvider("some-os", "dist", "arch"); err == nil {
		t.Fatalf("expected error for unsupported provider")
	}
}
