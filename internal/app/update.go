package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Goalt/personal-server/internal/logger"
)

const (
	githubAPIReleases = "https://api.github.com/repos/Goalt/personal-server/releases/latest"
)

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// UpdateChecker checks for updates
type UpdateChecker struct {
	logger       logger.Logger
	currentVer   string
	httpClient   *http.Client
	executableFn func() (string, error)
}

// NewUpdateChecker creates a new UpdateChecker
func NewUpdateChecker(log logger.Logger, currentVer string) *UpdateChecker {
	return &UpdateChecker{
		logger:       log,
		currentVer:   currentVer,
		httpClient:   &http.Client{},
		executableFn: os.Executable,
	}
}

// CheckAndUpdate checks for a new version and updates if available
func (u *UpdateChecker) CheckAndUpdate(ctx context.Context) error {
	u.logger.Info("🔍 Checking for updates...\n")
	u.logger.Info("Current version: %s\n", u.currentVer)

	// Fetch latest release info from GitHub
	release, err := u.fetchLatestRelease(ctx)
	if err != nil {
		return fmt.Errorf("fetching latest release: %w", err)
	}

	latestVersion := release.TagName
	u.logger.Info("Latest version: %s\n", latestVersion)

	// Compare versions
	if u.currentVer == latestVersion || u.currentVer == strings.TrimPrefix(latestVersion, "v") {
		u.logger.Info("✅ You are already running the latest version!\n")
		return nil
	}

	// Check if current version is "dev"
	if u.currentVer == "dev" {
		u.logger.Info("⚠️  You are running a development version.\n")
		u.logger.Info("Latest stable version available: %s\n", latestVersion)
		u.logger.Info("To update, download from: https://github.com/Goalt/personal-server/releases/latest\n")
		return nil
	}

	u.logger.Info("📦 New version available: %s\n", latestVersion)

	// Determine the appropriate binary for this platform
	assetName := u.getBinaryName()
	downloadURL := ""
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no binary found for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, latestVersion)
	}

	u.logger.Info("📥 Downloading %s...\n", assetName)

	// Download the new binary
	tmpFile, err := u.downloadBinary(ctx, downloadURL)
	if err != nil {
		return fmt.Errorf("downloading binary: %w", err)
	}
	defer os.Remove(tmpFile)

	// Replace the current binary
	if err := u.replaceBinary(tmpFile); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}

	u.logger.Info("✅ Successfully updated to version %s!\n", latestVersion)
	u.logger.Info("Please restart the application to use the new version.\n")

	return nil
}

// fetchLatestRelease fetches the latest release from GitHub API
func (u *UpdateChecker) fetchLatestRelease(ctx context.Context) (*GitHubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", githubAPIReleases, nil)
	if err != nil {
		return nil, err
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// getBinaryName returns the expected binary name for the current platform
func (u *UpdateChecker) getBinaryName() string {
	baseName := fmt.Sprintf("personal-server-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		baseName += ".exe"
	}
	return baseName
}

// downloadBinary downloads the binary to a temporary file
func (u *UpdateChecker) downloadBinary(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "personal-server-update-*")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	// Download to temp file
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	// Make it executable
	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

// replaceBinary replaces the current binary with the new one
func (u *UpdateChecker) replaceBinary(newBinaryPath string) error {
	// Get current executable path
	currentPath, err := u.executableFn()
	if err != nil {
		return err
	}

	// Resolve symlinks
	currentPath, err = filepath.EvalSymlinks(currentPath)
	if err != nil {
		return err
	}

	// Create a backup of the current binary
	backupPath := currentPath + ".backup"
	if err := os.Rename(currentPath, backupPath); err != nil {
		return fmt.Errorf("creating backup: %w", err)
	}

	// Copy new binary to current path
	if err := u.copyFile(newBinaryPath, currentPath); err != nil {
		// Restore backup on failure
		os.Rename(backupPath, currentPath)
		return fmt.Errorf("copying new binary: %w", err)
	}

	// Remove backup
	os.Remove(backupPath)

	return nil
}

// copyFile copies a file from src to dst
func (u *UpdateChecker) copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return err
	}

	// Copy permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}
