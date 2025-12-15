package sshlogin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
)

const (
	sshrcPath       = "/etc/ssh/sshrc"
	sshrcBackupPath = "/etc/ssh/sshrc.backup"
)

type SSHLoginModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *SSHLoginModule {
	return &SSHLoginModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *SSHLoginModule) Name() string {
	return "ssh-login-notifier"
}

func (m *SSHLoginModule) Generate(ctx context.Context) error {
	m.log.Info("SSH Login Notifier uses direct host installation.\n")
	m.log.Info("No Kubernetes manifests to generate.\n")
	return nil
}

func (m *SSHLoginModule) Apply(ctx context.Context) error {
	m.log.Info("Installing SSH login notification script...\n")
	m.log.Info("Target: %s\n\n", sshrcPath)

	// Get sentry DSN from config
	dsn, exists := m.ModuleConfig.Secrets["sentry_dsn"]
	if !exists || dsn == "" {
		return fmt.Errorf("sentry_dsn not found in module secrets")
	}

	// Get the path to the current binary
	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get binary path: %w", err)
	}

	// Get config file path from environment or use default
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		configPath = filepath.Join(cwd, "config.yaml")
	} else {
		// Convert relative path to absolute
		if !filepath.IsAbs(configPath) {
			absPath, err := filepath.Abs(configPath)
			if err != nil {
				return fmt.Errorf("failed to get absolute path for config: %w", err)
			}
			configPath = absPath
		}
	}

	// Check if script already exists
	if _, err := os.Stat(sshrcPath); err == nil {
		m.log.Warn("Script already exists at %s\n", sshrcPath)

		// Create backup
		m.log.Progress("Creating backup...\n")
		content, err := os.ReadFile(sshrcPath)
		if err != nil {
			return fmt.Errorf("failed to read existing script: %w", err)
		}

		if err := os.WriteFile(sshrcBackupPath, content, 0755); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
		m.log.Success("Backup created: %s\n", sshrcBackupPath)
	}

	// Generate script content
	scriptContent := m.generateScript(binaryPath, configPath)

	// Write script
	m.log.Progress("Writing script to %s...\n", sshrcPath)
	if err := os.WriteFile(sshrcPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write script: %w", err)
	}

	m.log.Success("Script installed successfully!\n")
	m.log.Info("\nSSH login notifications are now active.\n")
	m.log.Info("Test with: %s test\n", m.Name())

	return nil
}

func (m *SSHLoginModule) Clean(ctx context.Context) error {
	m.log.Info("Removing SSH login notification script...\n")
	m.log.Info("Target: %s\n\n", sshrcPath)

	// Check if script exists
	if _, err := os.Stat(sshrcPath); os.IsNotExist(err) {
		m.log.Warn("Script not found at %s (already removed or never installed)\n", sshrcPath)
		return nil
	}

	// Check if backup exists
	if _, err := os.Stat(sshrcBackupPath); err == nil {
		m.log.Info("Backup found, restoring...\n")

		content, err := os.ReadFile(sshrcBackupPath)
		if err != nil {
			return fmt.Errorf("failed to read backup: %w", err)
		}

		if err := os.WriteFile(sshrcPath, content, 0755); err != nil {
			return fmt.Errorf("failed to restore backup: %w", err)
		}

		// Remove backup file
		if err := os.Remove(sshrcBackupPath); err != nil {
			m.log.Warn("Failed to remove backup file: %v\n", err)
		}

		m.log.Success("Restored from backup: %s\n", sshrcBackupPath)
	} else {
		// No backup, just remove
		m.log.Progress("Removing script...\n")
		if err := os.Remove(sshrcPath); err != nil {
			return fmt.Errorf("failed to remove script: %w", err)
		}
		m.log.Success("Script removed successfully\n")
	}

	m.log.Info("\nSSH login notifications are now disabled.\n")
	return nil
}

func (m *SSHLoginModule) Status(ctx context.Context) error {
	m.log.Info("Checking SSH Login Notifier status...\n\n")

	// Check if script exists
	info, err := os.Stat(sshrcPath)
	if os.IsNotExist(err) {
		m.log.Error("Script NOT installed\n")
		m.log.Info("  Location: %s\n", sshrcPath)
		m.log.Info("  Run '%s apply' to install\n", m.Name())
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check script: %w", err)
	}

	m.log.Success("Script INSTALLED\n")
	m.log.Info("  Location: %s\n", sshrcPath)
	m.log.Info("  Size: %d bytes\n", info.Size())
	m.log.Info("  Permissions: %s\n", info.Mode().String())
	m.log.Info("  Modified: %s\n", info.ModTime().Format(time.RFC3339))

	// Check if backup exists
	if _, err := os.Stat(sshrcBackupPath); err == nil {
		m.log.Info("\nBackup exists: %s\n", sshrcBackupPath)
	}

	// Check if DSN is configured
	if dsn, exists := m.ModuleConfig.Secrets["sentry_dsn"]; !exists || dsn == "" || dsn == "abc" {
		m.log.Warn("\nSentry DSN not configured (using placeholder)\n")
		m.log.Info("  Update config.yaml with your Sentry DSN\n")
	} else {
		m.log.Success("\nSentry DSN configured\n")
	}

	return nil
}

func (m *SSHLoginModule) Test(ctx context.Context) error {
	m.log.Info("Sending test Sentry event...\n\n")

	// Get sentry DSN from config
	dsn, exists := m.ModuleConfig.Secrets["sentry_dsn"]
	if !exists || dsn == "" {
		return fmt.Errorf("sentry_dsn not found in module secrets")
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Send test event
	if err := m.sendSentryEvent(dsn, "SSH Login Notification Test", map[string]interface{}{
		"server":    hostname,
		"timestamp": time.Now().Format(time.RFC3339),
		"test":      true,
	}); err != nil {
		m.log.Error("Failed to send test event: %v\n", err)
		return err
	}

	m.log.Success("Test event sent successfully!\n")
	m.log.Info("Check your Sentry dashboard to verify receipt.\n")

	return nil
}

// Notify handles actual SSH login events (called by the installed script)
func (m *SSHLoginModule) Notify(ctx context.Context, user, ip, sshConnection string) error {
	// Get sentry DSN from config
	dsn, exists := m.ModuleConfig.Secrets["sentry_dsn"]
	if !exists || dsn == "" {
		return fmt.Errorf("sentry_dsn not found in module secrets")
	}

	// Get hostname
	hostname, _ := os.Hostname()

	// Create message
	message := fmt.Sprintf("SSH Login: %s from %s", user, ip)

	// Send event to Sentry
	if err := m.sendSentryEvent(dsn, message, map[string]interface{}{
		"user":           user,
		"ip":             ip,
		"ssh_connection": sshConnection,
		"server":         hostname,
		"login_time":     time.Now().Format(time.RFC3339),
	}); err != nil {
		return err
	}

	return nil
}

func (m *SSHLoginModule) generateScript(binaryPath, configPath string) string {
	return fmt.Sprintf(`#!/bin/bash

# SSH Login Sentry Notification Script
# Managed by personal-server

IP=$(echo $SSH_CONNECTION | cut -d " " -f 1)
logger -t ssh-wrapper "$USER login from $IP"

# Call personal-server binary to send notification
%s -c %s ssh-login-notifier notify "$USER" "$IP" "$SSH_CONNECTION" >/dev/null 2>&1 &

echo "$(date '+%%Y-%%m-%%d %%H:%%M:%%S') - SSH login: $USER from $IP" >> /var/log/ssh-logins.log 2>/dev/null || true
`, binaryPath, configPath)
}

func (m *SSHLoginModule) sendSentryEvent(dsn, message string, extra map[string]interface{}) error {
	// Parse DSN to extract components
	// DSN format: https://{key}@{org}.ingest.sentry.io/{project}

	// Remove https:// prefix
	if !strings.HasPrefix(dsn, "https://") {
		return fmt.Errorf("invalid Sentry DSN: must start with https://")
	}
	dsn = strings.TrimPrefix(dsn, "https://")

	// Split by @ to get key and rest
	parts := strings.SplitN(dsn, "@", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid Sentry DSN format: missing @ separator")
	}
	sentryKey := parts[0]

	// Split the rest by / to get host and project
	hostAndProject := strings.SplitN(parts[1], "/", 2)
	if len(hostAndProject) != 2 {
		return fmt.Errorf("invalid Sentry DSN format: missing project ID")
	}
	sentryHost := hostAndProject[0]
	sentryProject := hostAndProject[1]

	// Get hostname
	hostname, _ := os.Hostname()

	// Generate proper UUID for event_id (32-char hex, no dashes)
	eventID := strings.Replace(fmt.Sprintf("%032x", time.Now().UnixNano()), "-", "", -1)
	if len(eventID) > 32 {
		eventID = eventID[:32]
	}

	// Create event with Sentry v7 API format
	now := time.Now().UTC()
	event := map[string]interface{}{
		"event_id":    eventID,
		"timestamp":   now.Unix(), // Unix timestamp
		"platform":    "other",
		"level":       "info",
		"logger":      "ssh-login",
		"message":     message,
		"server_name": hostname,
		"tags":        map[string]string{"event_type": "test"},
		"extra":       extra,
	}

	// Marshal to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Send to Sentry using envelope format
	// Envelope format: {headers}\n{item_header}\n{payload}
	envelopeHeaders := map[string]interface{}{
		"event_id": eventID,
		"sent_at":  now.Format(time.RFC3339),
	}
	envelopeHeadersJSON, _ := json.Marshal(envelopeHeaders)

	itemHeader := map[string]interface{}{
		"type": "event",
	}
	itemHeaderJSON, _ := json.Marshal(itemHeader)

	// Build envelope: line1=envelope headers, line2=item header, line3=event
	envelope := fmt.Sprintf("%s\n%s\n%s", string(envelopeHeadersJSON), string(itemHeaderJSON), string(eventJSON))

	// Send to Sentry envelope endpoint
	apiURL := fmt.Sprintf("https://%s/api/%s/envelope/", sentryHost, sentryProject)
	req, err := http.NewRequest("POST", apiURL, bytes.NewBufferString(envelope))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-sentry-envelope")
	req.Header.Set("X-Sentry-Auth", fmt.Sprintf("Sentry sentry_version=7, sentry_key=%s, sentry_client=ssh-login-notifier/1.0", sentryKey))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Sentry API returned status %d", resp.StatusCode)
	}

	return nil
}
