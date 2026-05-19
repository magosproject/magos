package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateSourceDirAllowsMissingTerraformDir(t *testing.T) {
	sourceDir := t.TempDir()

	if err := validateSourceDir(sourceDir, ""); err != nil {
		t.Fatalf("validateSourceDir returned error for existing source dir without .terraform: %v", err)
	}
}

func TestValidateSourceDirFailsWhenSourceDirMissing(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "missing")

	err := validateSourceDir(sourceDir, "")
	if err == nil {
		t.Fatal("validateSourceDir succeeded for missing source dir")
	}
}

func TestValidateSourceDirFailsOnStatError(t *testing.T) {
	parent := t.TempDir()
	sourceDir := filepath.Join(parent, "source")
	if err := os.WriteFile(sourceDir, []byte("not a dir"), 0o600); err != nil {
		t.Fatalf("write source sentinel: %v", err)
	}

	err := validateSourceDir(sourceDir, "")
	if err == nil {
		t.Fatal("validateSourceDir succeeded for non-directory source path")
	}
}
