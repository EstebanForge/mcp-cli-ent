package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// autoInstallAlias attempts to install a "mcpclient" alias pointing to the current executable.
// It only installs if "mcpclient" cannot be found in the user's PATH,
// and places it in the first writable absolute directory in PATH.
func autoInstallAlias() {
	aliasName := "mcpclient"
	if runtime.GOOS == "windows" {
		aliasName = "mcpclient.cmd"
	}

	// 1. Check if "mcpclient" already exists in PATH
	if _, err := exec.LookPath("mcpclient"); err == nil {
		return
	}

	// 2. Get the current executable path
	selfPath, err := os.Executable()
	if err != nil {
		return
	}

	// 3. Find a writable directory in PATH to place the alias
	paths := filepath.SplitList(os.Getenv("PATH"))
	for _, p := range paths {
		if p == "" || !filepath.IsAbs(p) {
			continue
		}

		// Ensure the directory exists
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			continue
		}

		aliasPath := filepath.Join(p, aliasName)

		// If a file or symlink already exists at aliasPath, skip this directory
		if _, err := os.Lstat(aliasPath); err == nil {
			continue
		}

		// Verify write permission by attempting to create a temporary test file
		testFile := filepath.Join(p, fmt.Sprintf(".mcpclient_test_%d", os.Getpid()))
		f, err := os.OpenFile(testFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			continue
		}
		_ = f.Close()
		_ = os.Remove(testFile)

		// Directory is writable. Install the alias.
		if runtime.GOOS == "windows" {
			// On Windows, create a batch wrapper pointing to selfPath
			batContent := fmt.Sprintf("@echo off\n\"%s\" %%*\n", selfPath)
			if err := os.WriteFile(aliasPath, []byte(batContent), 0755); err == nil {
				break
			}
		} else {
			// On Unix/Linux/macOS, create a symlink
			if err := os.Symlink(selfPath, aliasPath); err == nil {
				break
			}
		}
	}
}
