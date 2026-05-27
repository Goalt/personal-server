package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain ensures the shared e2e test configuration file has secure
// permissions (0600) before tests run. The repository checks the file in
// with the default 0644 permissions, but the application refuses to load
// configuration files that are readable by group or other users.
func TestMain(m *testing.M) {
	candidates := []string{
		testConfigPath,
		filepath.Join("..", "..", testConfigPath),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if err := os.Chmod(p, 0o600); err != nil {
				panic("failed to chmod test config " + p + ": " + err.Error())
			}
		}
	}

	os.Exit(m.Run())
}
