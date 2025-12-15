package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/modules"
	"github.com/emersion/go-webdav"
	"github.com/getsentry/sentry-go"
)

func (a *App) handleGlobalBackupCommand(ctx context.Context, cfg *config.Config) error {
	// Initialize Sentry if DSN is provided
	if cfg.Backup.SentryDSN != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn: cfg.Backup.SentryDSN,
		})
		if err != nil {
			a.logger.Warn("Failed to initialize Sentry: %v\n", err)
		} else {
			a.logger.Info("✅ Sentry initialized for error tracking\n")
			defer sentry.Flush(2 * time.Second)
		}
	}

	timestamp := time.Now().Format("20060102_150405")
	globalBackupDir := filepath.Join("backups", fmt.Sprintf("global_backup_%s", timestamp))

	a.logger.Info("🚀 Starting global backup...\n")
	a.logger.Info("Directory: %s\n\n", globalBackupDir)

	if err := os.MkdirAll(globalBackupDir, 0755); err != nil {
		if cfg.Backup.SentryDSN != "" {
			sentry.CaptureException(err)
		}
		return fmt.Errorf("failed to create global backup directory: %w", err)
	}

	successCount := 0
	failCount := 0

	// Iterate over all available modules
	moduleNames := a.registry.Commands()
	for _, name := range moduleNames {
		module, err := a.registry.Get(name, cfg)
		if err != nil {
			a.logger.Warn("Failed to load module '%s': %v\n", name, err)
			continue
		}

		if backuper, ok := module.(modules.Backuper); ok {
			a.logger.Info("📦 Backing up module: %s\n", name)
			if err := backuper.Backup(ctx, globalBackupDir); err != nil {
				if cfg.Backup.SentryDSN != "" {
					sentry.CaptureException(err)
				}
				a.logger.Error("❌ Failed to backup module '%s': %v\n", name, err)
				failCount++
			} else {
				a.logger.Success("✅ Module '%s' backed up successfully\n", name)
				successCount++
			}
			a.logger.Println()
		}
	}

	a.logger.Info("Global backup summary: %d successful, %d failed\n", successCount, failCount)

	if successCount == 0 {
		err := fmt.Errorf("no backups were created")
		if cfg.Backup.SentryDSN != "" {
			sentry.CaptureException(err)
		}
		return err
	}

	// Include current binary
	exePath, err := os.Executable()
	if err != nil {
		a.logger.Warn("Failed to get executable path: %v\n", err)
	} else {
		a.logger.Info("📦 Including binary in backup: %s\n", exePath)
		// Resolve symlinks just in case
		realPath, err := filepath.EvalSymlinks(exePath)
		if err != nil {
			a.logger.Warn("Failed to resolve symlink for executable: %v\n", err)
			realPath = exePath
		}

		destPath := filepath.Join(globalBackupDir, filepath.Base(realPath))
		srcFile, err := os.Open(realPath)
		if err != nil {
			a.logger.Warn("Failed to open executable: %v\n", err)
		} else {
			defer srcFile.Close()
			destFile, err := os.Create(destPath)
			if err != nil {
				a.logger.Warn("Failed to create backup binary file: %v\n", err)
			} else {
				defer destFile.Close()
				if _, err := io.Copy(destFile, srcFile); err != nil {
					a.logger.Warn("Failed to copy executable: %v\n", err)
				} else {
					// Preserve permissions (executable)
					if info, err := srcFile.Stat(); err == nil {
						os.Chmod(destPath, info.Mode())
					}
					a.logger.Success("✅ Binary included\n")
				}
			}
		}
		a.logger.Println()
	}

	// Include config file
	if cfg.Path != "" {
		a.logger.Info("📦 Including config file in backup: %s\n", cfg.Path)
		destPath := filepath.Join(globalBackupDir, filepath.Base(cfg.Path))
		srcFile, err := os.Open(cfg.Path)
		if err != nil {
			a.logger.Warn("Failed to open config file: %v\n", err)
		} else {
			defer srcFile.Close()
			destFile, err := os.Create(destPath)
			if err != nil {
				a.logger.Warn("Failed to create backup config file: %v\n", err)
			} else {
				defer destFile.Close()
				if _, err := io.Copy(destFile, srcFile); err != nil {
					a.logger.Warn("Failed to copy config file: %v\n", err)
				} else {
					a.logger.Success("✅ Config file included\n")
				}
			}
		}
		a.logger.Println()
	}

	// Create tar archive
	archiveFile := fmt.Sprintf("%s.tar.gz", globalBackupDir)
	a.logger.Info("📦 Creating archive: %s\n", archiveFile)

	// tar -czf backups/global_backup_TIMESTAMP.tar.gz -C backups global_backup_TIMESTAMP
	cmd := exec.CommandContext(ctx, "tar", "-czf", archiveFile, "-C", "backups", filepath.Base(globalBackupDir))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if cfg.Backup.SentryDSN != "" {
			sentry.CaptureException(err)
		}
		return fmt.Errorf("failed to create archive: %w", err)
	}

	// Encrypt the archive
	encryptedArchiveFile := archiveFile + ".gpg"
	a.logger.Info("🔒 Encrypting archive: %s\n", encryptedArchiveFile)

	// gpg --batch --yes --passphrase-fd 0 --symmetric --cipher-algo AES256 -o <archive>.gpg <archive>
	gpgCmd := exec.CommandContext(ctx, "gpg", "--batch", "--yes", "--passphrase-fd", "0", "--symmetric", "--cipher-algo", "AES256", "-o", encryptedArchiveFile, archiveFile)

	stdin, err := gpgCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe for gpg: %w", err)
	}

	gpgCmd.Stdout = os.Stdout
	gpgCmd.Stderr = os.Stderr

	if err := gpgCmd.Start(); err != nil {
		return fmt.Errorf("failed to start gpg command: %w", err)
	}

	// Write passphrase to stdin (with newline terminator)
	if _, err := io.WriteString(stdin, cfg.Backup.Passphrase+"\n"); err != nil {
		return fmt.Errorf("failed to write passphrase to gpg stdin: %w", err)
	}
	stdin.Close()

	if err := gpgCmd.Wait(); err != nil {
		if cfg.Backup.SentryDSN != "" {
			sentry.CaptureException(err)
		}
		return fmt.Errorf("failed to encrypt archive: %w", err)
	}

	// Remove the unencrypted archive
	if err := os.Remove(archiveFile); err != nil {
		a.logger.Warn("Failed to remove unencrypted archive: %v\n", err)
	}

	// Remove the backup directory
	if err := os.RemoveAll(globalBackupDir); err != nil {
		a.logger.Warn("Failed to remove backup directory: %v\n", err)
	}

	a.logger.Success("\n🎉 Global backup complete! Encrypted archive: %s\n", encryptedArchiveFile)

	if err := a.uploadToWebDAV(ctx, encryptedArchiveFile, cfg.Backup.WebdavHost, cfg.Backup.WebdavUsername, cfg.Backup.WebdavPassword); err != nil {
		if cfg.Backup.SentryDSN != "" {
			sentry.CaptureException(err)
		}
		return fmt.Errorf("failed to upload to WebDAV: %w", err)
	}

	// Remove the encrypted archive after successful upload
	if err := os.Remove(encryptedArchiveFile); err != nil {
		a.logger.Warn("Failed to remove encrypted archive: %v\n", err)
	}

	// Capture success event in Sentry with detailed information
	if cfg.Backup.SentryDSN != "" {
		// Get file info for the encrypted archive
		fileInfo, err := os.Stat(encryptedArchiveFile)
		var filesSize int64
		if err == nil {
			filesSize = fileInfo.Size()
		}

		// Build list of included files
		includedFiles := []string{}
		for _, name := range moduleNames {
			if module, err := a.registry.Get(name, cfg); err == nil {
				if _, ok := module.(modules.Backuper); ok {
					includedFiles = append(includedFiles, fmt.Sprintf("module:%s", name))
				}
			}
		}
		// Add binary and config if they were included
		if exePath, err := os.Executable(); err == nil {
			includedFiles = append(includedFiles, fmt.Sprintf("binary:%s", filepath.Base(exePath)))
		}
		if cfg.Path != "" {
			includedFiles = append(includedFiles, fmt.Sprintf("config:%s", filepath.Base(cfg.Path)))
		}

		sentry.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetContext("backup_info", map[string]interface{}{
				"archive_name":   filepath.Base(encryptedArchiveFile),
				"included_files": includedFiles,
				"files_size":     filesSize,
				"files_size_mb":  float64(filesSize) / (1024 * 1024),
				"success_count":  successCount,
				"fail_count":     failCount,
				"timestamp":      timestamp,
			})
		})
		sentry.CaptureMessage(fmt.Sprintf("Backup completed successfully: %s", filepath.Base(encryptedArchiveFile)))
	}

	return nil
}

func (a *App) uploadToWebDAV(ctx context.Context, filePath, host, username, password string) error {
	a.logger.Info("☁️ Uploading to WebDAV: %s\n", host)

	// Create WebDAV client
	transport := &http.Transport{}
	client := &http.Client{
		Transport: &basicAuthTransport{
			Username:  username,
			Password:  password,
			Transport: transport,
		},
	}

	wdClient, err := webdav.NewClient(client, host)
	if err != nil {
		return fmt.Errorf("failed to create webdav client: %w", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for upload: %w", err)
	}
	defer file.Close()

	// Upload file
	remotePath := filepath.Base(filePath)
	wc, err := wdClient.Create(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer wc.Close()

	if _, err := io.Copy(wc, file); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	a.logger.Success("✅ Uploaded %s to WebDAV\n", remotePath)
	return nil
}

type basicAuthTransport struct {
	Username  string
	Password  string
	Transport http.RoundTripper
}

func (t *basicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(t.Username, t.Password)
	return t.Transport.RoundTrip(req)
}

func (a *App) downloadFromWebDAV(ctx context.Context, remotePath, localPath, host, username, password string) error {
	a.logger.Info("☁️ Downloading from WebDAV: %s\n", host)

	// Create WebDAV client
	transport := &http.Transport{}
	client := &http.Client{
		Transport: &basicAuthTransport{
			Username:  username,
			Password:  password,
			Transport: transport,
		},
	}

	wdClient, err := webdav.NewClient(client, host)
	if err != nil {
		return fmt.Errorf("failed to create webdav client: %w", err)
	}

	// Open remote file for reading
	rc, err := wdClient.Open(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer rc.Close()

	// Create local file
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer file.Close()

	// Copy content from remote to local
	if _, err := io.Copy(file, rc); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	a.logger.Success("✅ Downloaded %s to %s\n", remotePath, localPath)
	return nil
}

func (a *App) handleBackupDownload(ctx context.Context, cfg *config.Config, remotePath string) error {
	a.logger.Info("📥 Starting backup download...\n")

	// Clean and validate the remote path to prevent path traversal
	remotePath = filepath.Clean(remotePath)

	// Use current directory for downloaded file, using only the base name
	localPath := filepath.Base(remotePath)

	// Additional validation to ensure filename is safe
	if localPath == "." || localPath == ".." || localPath == "/" {
		return fmt.Errorf("invalid filename: %s", remotePath)
	}

	// Check if file already exists
	if _, err := os.Stat(localPath); err == nil {
		return fmt.Errorf("file %s already exists, please remove it first or use a different name", localPath)
	}

	if err := a.downloadFromWebDAV(ctx, remotePath, localPath, cfg.Backup.WebdavHost, cfg.Backup.WebdavUsername, cfg.Backup.WebdavPassword); err != nil {
		return fmt.Errorf("failed to download from WebDAV: %w", err)
	}

	a.logger.Success("\n🎉 Backup download complete! File saved as: %s\n", localPath)
	return nil
}

func (a *App) handleGlobalDecryptCommand(ctx context.Context, archivePath string, passphrase string) error {
	a.logger.Info("🔓 Decrypting archive: %s\n", archivePath)

	// gpg --batch --yes --passphrase-fd 0 --decrypt <archivePath> | tar -xz
	gpgCmd := exec.CommandContext(ctx, "gpg", "--batch", "--yes", "--passphrase-fd", "0", "--decrypt", archivePath)
	tarCmd := exec.CommandContext(ctx, "tar", "-xz")

	// Pipe gpg output to tar input
	gpgStdout, err := gpgCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create gpg stdout pipe: %w", err)
	}
	tarCmd.Stdin = gpgStdout

	// Set output for tar execution
	tarCmd.Stdout = os.Stdout
	tarCmd.Stderr = os.Stderr

	// Pipe passphrase to gpg stdin
	gpgStdin, err := gpgCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create gpg stdin pipe: %w", err)
	}

	if err := gpgCmd.Start(); err != nil {
		return fmt.Errorf("failed to start gpg command: %w", err)
	}

	// Write passphrase (with newline terminator)
	if _, err := io.WriteString(gpgStdin, passphrase+"\n"); err != nil {
		return fmt.Errorf("failed to write passphrase to gpg stdin: %w", err)
	}
	gpgStdin.Close()

	if err := tarCmd.Start(); err != nil {
		return fmt.Errorf("failed to start tar command: %w", err)
	}

	// Wait for commands to finish
	if err := gpgCmd.Wait(); err != nil {
		return fmt.Errorf("failed to decrypt archive: %w", err)
	}

	if err := tarCmd.Wait(); err != nil {
		return fmt.Errorf("failed to untar archive: %w", err)
	}

	a.logger.Success("✅ Archive decrypted and extracted successfully\n")
	return nil
}

func (a *App) handleBackupSchedule(ctx context.Context, cfg *config.Config) error {
	a.logger.Info("📅 Scheduling backup job...\n")

	// Get absolute path to executable
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks for executable: %w", err)
	}
	exeAbsPath, err := filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for executable: %w", err)
	}

	// Get absolute path to config file
	configAbsPath, err := filepath.Abs(cfg.Path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for configuration file: %w", err)
	}

	// Construct the command
	// personal-server --config /path/to/config.yaml backup
	cmdStr := fmt.Sprintf("%s --config %s backup",
		exeAbsPath, configAbsPath)

	// Construct cron line
	// */5 * * * * /path/to/personal-server ... # personal-server-backup-job
	cronLine := fmt.Sprintf("%s %s # personal-server-backup-job", cfg.Backup.Cron, cmdStr)

	a.logger.Info("Cron Job: %s\n", cronLine)

	// Read current crontab
	readCmd := exec.CommandContext(ctx, "crontab", "-l")
	currentCrontab, err := readCmd.Output()
	if err != nil {
		// If no crontab exists, it might exit with status 1, which is fine, we just start with empty
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			currentCrontab = []byte{}
		} else {
			// For other errors, we might want to just proceed if it's "no crontab for user"
			// but depending on OS/cron implementation it varies.
			// Let's assume empty if it fails.
			a.logger.Warn("Failed to read current crontab (might be empty): %v\n", err)
			currentCrontab = []byte{}
		}
	}

	// Append new job
	newCrontab := string(currentCrontab)
	if len(newCrontab) > 0 && newCrontab[len(newCrontab)-1] != '\n' {
		newCrontab += "\n"
	}
	newCrontab += cronLine + "\n"

	// Write back to crontab
	writeCmd := exec.CommandContext(ctx, "crontab", "-")

	stdin, err := writeCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe for crontab: %w", err)
	}

	writeCmd.Stdout = os.Stdout
	writeCmd.Stderr = os.Stderr

	if err := writeCmd.Start(); err != nil {
		return fmt.Errorf("failed to start crontab write: %w", err)
	}

	if _, err := io.WriteString(stdin, newCrontab); err != nil {
		return fmt.Errorf("failed to write to crontab stdin: %w", err)
	}
	stdin.Close()

	if err := writeCmd.Wait(); err != nil {
		return fmt.Errorf("failed to update crontab: %w", err)
	}

	a.logger.Success("✅ Backup schedule added to crontab\n")
	return nil
}
