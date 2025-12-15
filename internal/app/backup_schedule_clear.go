package app

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func (a *App) handleBackupScheduleClear(ctx context.Context) error {
	a.logger.Info("🗑️  Clearing backup schedule...\n")

	// Read current crontab
	readCmd := exec.CommandContext(ctx, "crontab", "-l")
	currentCrontab, err := readCmd.Output()
	if err != nil {
		// If no crontab exists, it might exit with status 1
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			a.logger.Warn("No crontab found for current user\n")
			return nil
		}
		return fmt.Errorf("failed to read current crontab: %w", err)
	}

	// Filter out the backup job line
	lines := strings.Split(string(currentCrontab), "\n")
	var newLines []string
	removed := false

	for _, line := range lines {
		// Skip the line if it contains the backup job tag
		if strings.Contains(line, "# personal-server-backup-job") {
			removed = true
			a.logger.Info("Removing: %s\n", line)
			continue
		}
		// Keep all other lines
		if line != "" || len(newLines) > 0 {
			newLines = append(newLines, line)
		}
	}

	if !removed {
		a.logger.Warn("No backup schedule found in crontab\n")
		return nil
	}

	// Write back the filtered crontab
	newCrontab := strings.Join(newLines, "\n")

	writeCmd := exec.CommandContext(ctx, "crontab", "-")
	stdin, err := writeCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe for crontab: %w", err)
	}

	if err := writeCmd.Start(); err != nil {
		return fmt.Errorf("failed to start crontab write: %w", err)
	}

	if _, err := stdin.Write([]byte(newCrontab)); err != nil {
		return fmt.Errorf("failed to write to crontab stdin: %w", err)
	}
	stdin.Close()

	if err := writeCmd.Wait(); err != nil {
		return fmt.Errorf("failed to update crontab: %w", err)
	}

	a.logger.Success("✅ Backup schedule cleared from crontab\n")
	return nil
}
