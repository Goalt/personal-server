package config

import (
	"fmt"
	"os"
	"runtime"
)

// checkConfigPermissions validates that the configuration file is not
// readable or writable by group or other users. The configuration file
// can contain secrets (passwords, tokens, credentials), so its
// permissions must be restricted to the owner.
//
// On Unix-like systems, only file modes 0600 and 0400 are accepted.
// On Windows the Unix permission model does not apply, so the check is
// skipped.
func checkConfigPermissions(path string, mode os.FileMode) error {
	if runtime.GOOS == "windows" {
		return nil
	}

	// Reject any permission bits set for group or other.
	if mode&0o077 != 0 {
		return fmt.Errorf(
			"config file %s has insecure permissions %#o: it may contain secrets "+
				"and must not be accessible to group or other users; "+
				"set permissions to 0600 (e.g. chmod 600 %s)",
			path, mode, path,
		)
	}

	return nil
}
