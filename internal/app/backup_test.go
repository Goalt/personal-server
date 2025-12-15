package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
	"github.com/Goalt/personal-server/internal/modules"
)

func TestUploadToWebDAV(t *testing.T) {
	// Create a temporary file to upload
	tmpFile, err := os.CreateTemp("", "backup-test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := []byte("test content")
	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Create a mock WebDAV server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check Basic Auth
		username, password, ok := r.BasicAuth()
		if !ok || username != "user" || password != "pass" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Check method
		if r.Method != "PUT" {
			// go-webdav Create actually uses PUT under the hood for files
			// But let's log if it's something unexpected
			t.Logf("Received method: %s", r.Method)
		}

		// Expected URL path should be the filename
		expectedPath := "/" + filepath.Base(tmpFile.Name())
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if string(body) != string(content) {
			t.Errorf("Expected body %s, got %s", string(content), string(body))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	// Create App instance with mocked logger (optional, or just discard)
	// We need to use the actual App struct to call the method
	app := &App{
		logger:   logger.Default(),
		registry: modules.DefaultRegistry(logger.Default()), // minimal setup
	}

	// Test upload
	// Note: uploadToWebDAV is unexported, but we are in the same package (app)
	err = app.uploadToWebDAV(context.Background(), tmpFile.Name(), server.URL, "user", "pass")
	if err != nil {
		t.Errorf("uploadToWebDAV failed: %v", err)
	}
}

func TestDownloadFromWebDAV(t *testing.T) {
	content := []byte("test backup content")

	// Create a mock WebDAV server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check Basic Auth
		username, password, ok := r.BasicAuth()
		if !ok || username != "user" || password != "pass" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Check method (should be GET or PROPFIND for webdav)
		if r.Method != "GET" && r.Method != "PROPFIND" {
			t.Logf("Received method: %s", r.Method)
		}

		// For GET requests, return the content
		if r.Method == "GET" {
			expectedPath := "/backup.tar.gz.gpg"
			if r.URL.Path != expectedPath {
				t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
			}

			w.WriteHeader(http.StatusOK)
			w.Write(content)
		}
	}))
	defer server.Close()

	// Create App instance
	app := &App{
		logger:   logger.Default(),
		registry: modules.DefaultRegistry(logger.Default()),
	}

	// Create a temporary file for download
	tmpDir := t.TempDir()
	localPath := filepath.Join(tmpDir, "downloaded-backup.tar.gz.gpg")

	// Test download
	err := app.downloadFromWebDAV(context.Background(), "/backup.tar.gz.gpg", localPath, server.URL, "user", "pass")
	if err != nil {
		t.Errorf("downloadFromWebDAV failed: %v", err)
	}

	// Verify downloaded file content
	downloadedContent, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(downloadedContent) != string(content) {
		t.Errorf("Expected downloaded content %s, got %s", string(content), string(downloadedContent))
	}
}

func TestHandleBackupDownload_FileExists(t *testing.T) {
	content := []byte("test backup content")

	// Create a mock WebDAV server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check Basic Auth
		username, password, ok := r.BasicAuth()
		if !ok || username != "user" || password != "pass" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			w.Write(content)
		}
	}))
	defer server.Close()

	// Create App instance
	app := &App{
		logger:   logger.Default(),
		registry: modules.DefaultRegistry(logger.Default()),
	}

	// Create a temporary directory and change to it
	tmpDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create a file that will conflict
	existingFile := "backup.tar.gz.gpg"
	if err := os.WriteFile(existingFile, []byte("existing content"), 0644); err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	// Mock config
	cfg := &config.Config{
		Backup: config.BackupConfig{
			WebdavHost:     server.URL,
			WebdavUsername: "user",
			WebdavPassword: "pass",
		},
	}

	// Test download - should fail because file exists
	err = app.handleBackupDownload(context.Background(), cfg, "/backup.tar.gz.gpg")
	if err == nil {
		t.Error("Expected error when file exists, but got nil")
	} else {
		// Check if error message indicates file already exists
		expectedMsg := "file backup.tar.gz.gpg already exists, please remove it first or use a different name"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message containing file exists, got: %v", err)
		}
	}
}

func TestHandleBackupDownload_InvalidFilename(t *testing.T) {
	// Create App instance
	app := &App{
		logger:   logger.Default(),
		registry: modules.DefaultRegistry(logger.Default()),
	}

	// Mock config
	cfg := &config.Config{
		Backup: config.BackupConfig{
			WebdavHost:     "http://example.com",
			WebdavUsername: "user",
			WebdavPassword: "pass",
		},
	}

	// Test invalid filenames
	testCases := []string{
		".",
		"..",
		"/",
	}

	for _, tc := range testCases {
		err := app.handleBackupDownload(context.Background(), cfg, tc)
		if err == nil {
			t.Errorf("Expected error for invalid filename %s, but got nil", tc)
		}
	}
}
