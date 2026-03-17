package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBackupDecryptWithoutConfig verifies that the decrypt command works correctly
// when no config file exists. The command should NOT fail with a
// "config file not found" error — it should instead fail at the GPG/archive
// level because decryption does not require a config file.
func TestBackupDecryptWithoutConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	relBinaryPath := filepath.Join("..", "..", binaryPath)
	absBinaryPath, err := filepath.Abs(relBinaryPath)
	if err != nil {
		t.Fatalf("failed to resolve binary path: %v", err)
	}

	if _, err := os.Stat(absBinaryPath); os.IsNotExist(err) {
		t.Fatalf("binary not found at %s. Run 'make build' first", absBinaryPath)
	}

	// Run from a temporary directory that has no config.yaml, ensuring we test
	// the "no config file" scenario.
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// The archive file doesn't exist either — we just need to confirm the error
	// is NOT a config-loading error.
	output, runErr := runCommand(t, absBinaryPath,
		"backup",
		"--decrypt", "nonexistent-archive.tar.gz.gpg",
		"--passphrase", "testpassphrase",
	)

	// The command must fail (archive doesn't exist / GPG can't decrypt it).
	if runErr == nil {
		t.Fatalf("expected decrypt command to fail (no archive), but it succeeded. Output:\n%s", output)
	}

	// The error must NOT be about a missing config file.
	configErrMsg := "config file not found"
	if strings.Contains(output, configErrMsg) || strings.Contains(runErr.Error(), configErrMsg) {
		t.Errorf("decrypt command should not require a config file, but got config error.\nOutput: %s\nError: %v", output, runErr)
	}

	t.Logf("Decrypt command failed as expected (no archive, no config needed). Output:\n%s", output)
}

// TestBackupDecryptMissingPassphrase verifies that the decrypt command returns
// a proper error when --passphrase is omitted, without requiring a config file.
func TestBackupDecryptMissingPassphrase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	relBinaryPath := filepath.Join("..", "..", binaryPath)
	absBinaryPath, err := filepath.Abs(relBinaryPath)
	if err != nil {
		t.Fatalf("failed to resolve binary path: %v", err)
	}

	if _, err := os.Stat(absBinaryPath); os.IsNotExist(err) {
		t.Fatalf("binary not found at %s. Run 'make build' first", absBinaryPath)
	}

	// Run from a temporary directory without config.yaml.
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	output, runErr := runCommand(t, absBinaryPath,
		"backup",
		"--decrypt", "some-archive.tar.gz.gpg",
	)

	if runErr == nil {
		t.Fatalf("expected command to fail when passphrase is missing, but it succeeded. Output:\n%s", output)
	}

	// Must error with the passphrase message, not a config-not-found message.
	configErrMsg := "config file not found"
	if strings.Contains(output, configErrMsg) || strings.Contains(runErr.Error(), configErrMsg) {
		t.Errorf("decrypt command should not require a config file, but got config error.\nOutput: %s\nError: %v", output, runErr)
	}

	passphraseErrMsg := "passphrase is required"
	if !strings.Contains(output, passphraseErrMsg) && !strings.Contains(runErr.Error(), passphraseErrMsg) {
		t.Errorf("expected passphrase error, got:\nOutput: %s\nError: %v", output, runErr)
	}

	t.Logf("Decrypt command correctly required passphrase (no config needed). Output:\n%s", output)
}
