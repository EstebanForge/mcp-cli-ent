package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAutoInstallAlias(t *testing.T) {
	// Create a temp directory to use as our mock PATH
	tempDir := t.TempDir()

	// Set PATH to only contain our temp directory
	t.Setenv("PATH", tempDir)

	// Verify look path doesn't find it
	aliasName := "mcpclient"
	if runtime.GOOS == "windows" {
		aliasName = "mcpclient.cmd"
	}

	expectedPath := filepath.Join(tempDir, aliasName)

	// Ensure it doesn't exist yet
	if _, err := os.Lstat(expectedPath); err == nil {
		t.Fatalf("Expected alias path to not exist, but it does: %s", expectedPath)
	}

	// Run autoInstallAlias
	autoInstallAlias()

	// Verify alias was created
	if _, err := os.Lstat(expectedPath); err != nil {
		t.Fatalf("Expected alias path to be created at %s, but got error: %v", expectedPath, err)
	}

	if runtime.GOOS != "windows" {
		// Verify it is a symlink pointing to the current test executable
		linkTarget, err := os.Readlink(expectedPath)
		if err != nil {
			t.Fatalf("Failed to read symlink target: %v", err)
		}
		selfPath, err := os.Executable()
		if err != nil {
			t.Fatalf("Failed to get executable path: %v", err)
		}
		if linkTarget != selfPath {
			t.Errorf("Expected symlink target %s, got %s", selfPath, linkTarget)
		}
	} else {
		// On Windows, verify it is a batch/cmd file containing the current executable
		content, err := os.ReadFile(expectedPath)
		if err != nil {
			t.Fatalf("Failed to read batch file: %v", err)
		}
		selfPath, err := os.Executable()
		if err != nil {
			t.Fatalf("Failed to get executable path: %v", err)
		}
		expectedContent := "@echo off\n\"" + selfPath + "\" %*\n"
		if string(content) != expectedContent {
			t.Errorf("Expected content %q, got %q", expectedContent, string(content))
		}
	}

	// Now run it again. Since LookPath should find it (or the file exists), it should not fail/modify.
	// We check that running it again works and doesn't cause errors.
	autoInstallAlias()
}
