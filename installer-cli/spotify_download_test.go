package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownloadSpotifyInstallerStopsOnPermanentSizeMismatch(t *testing.T) {
	primaryCalls, fallbackCalls, sleeps := 0, 0, 0
	methods := []installerDownloadMethod{
		{name: "primary", run: func(string, string, int64, downloadLogger) error {
			primaryCalls++
			return &installerDownloadError{permanent: true, err: errors.New("server advertises 1024 bytes, catalog size 148153000 bytes")}
		}},
		{name: "fallback", run: func(string, string, int64, downloadLogger) error {
			fallbackCalls++
			return nil
		}},
	}
	err := downloadSpotifyInstaller("https://example.invalid/spotify.exe", filepath.Join(t.TempDir(), "spotify.exe"), 148153000, methods, nil, func(time.Duration) {
		sleeps++
	})
	if err == nil || !strings.Contains(err.Error(), "wrong size") {
		t.Fatalf("expected wrong-size advice, got %v", err)
	}
	if primaryCalls != 1 || fallbackCalls != 0 || sleeps != 0 {
		t.Fatalf("unexpected retries: primary=%d fallback=%d sleeps=%d", primaryCalls, fallbackCalls, sleeps)
	}
}

func TestDownloadInstallerHTTPRejectsWrongDeclaredSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024")
		_, _ = w.Write(make([]byte, 1024))
	}))
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "spotify.exe")
	err := downloadInstallerHTTP(server.Client(), server.URL, destination, 148153000, nil)
	var downloadErr *installerDownloadError
	if !errors.As(err, &downloadErr) || !downloadErr.permanent {
		t.Fatalf("expected permanent size mismatch, got %v", err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("unexpected destination file after rejected response: stat err=%v", err)
	}
}

func TestDownloadInstallerCurlRejectsWrongDeclaredSize(t *testing.T) {
	curlPath, err := exec.LookPath("curl.exe")
	if err != nil {
		t.Skip("curl.exe is not available")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024")
		_, _ = w.Write(make([]byte, 1024))
	}))
	defer server.Close()

	command := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, name, args...)
	}
	destination := filepath.Join(t.TempDir(), "spotify.exe")
	err = downloadInstallerCurl(command, curlPath, server.URL, destination, 148153000, nil)
	var downloadErr *installerDownloadError
	if !errors.As(err, &downloadErr) || !downloadErr.permanent {
		t.Fatalf("expected permanent size mismatch, got %v", err)
	}
}
