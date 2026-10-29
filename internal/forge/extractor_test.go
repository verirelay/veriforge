package forge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/verirelay/veriforge/internal/config"
)

func TestFileHasSHA256(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "artifact.deb")
	content := []byte("verified content")
	mustWriteFile(t, filePath, content, 0o644)

	if !fileHasSHA256(filePath, sha256Hex(content)) {
		t.Fatal("fileHasSHA256() = false, want true for matching hash")
	}
	if fileHasSHA256(filePath, sha256Hex([]byte("other content"))) {
		t.Fatal("fileHasSHA256() = true, want false for mismatched hash")
	}
	if fileHasSHA256(filepath.Join(t.TempDir(), "missing.deb"), sha256Hex(content)) {
		t.Fatal("fileHasSHA256() = true, want false for missing file")
	}
}

func TestDownloadAndVerifyStoresValidContent(t *testing.T) {
	content := []byte("downloaded artifact")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer server.Close()

	targetPath := filepath.Join(t.TempDir(), "artifact.deb")
	extractor := NewExtractor(config.KernelPackage{}, t.TempDir())
	if err := extractor.downloadAndVerify(t.Context(), server.URL, targetPath, sha256Hex(content)); err != nil {
		t.Fatalf("downloadAndVerify() error = %v", err)
	}

	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("downloaded content = %q, want %q", string(got), string(content))
	}
}

func TestDownloadAndVerifyRemovesInvalidContent(t *testing.T) {
	content := []byte("tampered artifact")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer server.Close()

	targetPath := filepath.Join(t.TempDir(), "artifact.deb")
	extractor := NewExtractor(config.KernelPackage{}, t.TempDir())
	err := extractor.downloadAndVerify(t.Context(), server.URL, targetPath, sha256Hex([]byte("expected artifact")))
	if err == nil {
		t.Fatal("downloadAndVerify() error = nil, want hash mismatch error")
	}
	if _, statErr := os.Stat(targetPath); !os.IsNotExist(statErr) {
		t.Fatalf("target file still exists after verification failure, statErr=%v", statErr)
	}
}

func TestDownloadAndVerifyCanceledBeforeRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("server received request with canceled context")
	}))
	defer server.Close()

	targetPath := filepath.Join(t.TempDir(), "artifact.deb")
	extractor := NewExtractor(config.KernelPackage{}, t.TempDir())
	err := extractor.downloadAndVerify(ctx, server.URL, targetPath, sha256Hex([]byte("artifact")))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("downloadAndVerify() error = %v, want context.Canceled", err)
	}
	if _, statErr := os.Stat(targetPath); !os.IsNotExist(statErr) {
		t.Fatalf("target file still exists after canceled download, statErr=%v", statErr)
	}
}

func TestDownloadAndVerifyCancelMidDownload(t *testing.T) {
	firstChunkSent := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("first chunk"))
		w.(http.Flusher).Flush()
		close(firstChunkSent)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	targetPath := filepath.Join(t.TempDir(), "artifact.deb")
	extractor := NewExtractor(config.KernelPackage{}, t.TempDir())
	errCh := make(chan error, 1)
	go func() {
		errCh <- extractor.downloadAndVerify(ctx, server.URL, targetPath, sha256Hex([]byte("first chunk")))
	}()

	select {
	case <-firstChunkSent:
		cancel()
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("server did not send the first download chunk")
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("downloadAndVerify() error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("downloadAndVerify() did not return after cancellation")
	}
	if _, statErr := os.Stat(targetPath); !os.IsNotExist(statErr) {
		t.Fatalf("target file still exists after canceled download, statErr=%v", statErr)
	}
}

func TestExtractVmlinuzUsesExactKernelRelease(t *testing.T) {
	workDir := t.TempDir()
	installFakeDpkgDeb(t, func(outDir string) error {
		bootDir := filepath.Join(outDir, "boot")
		if err := os.MkdirAll(bootDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(bootDir, "vmlinuz-test-2-generic"), []byte("wrong"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(bootDir, "vmlinuz-test-1-generic"), []byte("kernel"), 0o644)
	})

	destPath := filepath.Join(t.TempDir(), "nested", "vmlinuz")
	extractor := NewExtractor(config.KernelPackage{KernelRelease: "test-1-generic"}, workDir)
	if err := extractor.extractVmlinuz(filepath.Join(workDir, "kernel.deb"), destPath); err != nil {
		t.Fatalf("extractVmlinuz() error = %v", err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read vmlinuz: %v", err)
	}
	if string(got) != "kernel" {
		t.Fatalf("vmlinuz content = %q, want %q", string(got), "kernel")
	}
}

func TestExtractModulesUsesExactKernelRelease(t *testing.T) {
	workDir := t.TempDir()
	installFakeDpkgDeb(t, func(outDir string) error {
		modulesRoot := filepath.Join(outDir, "lib", "modules")
		if err := os.MkdirAll(filepath.Join(modulesRoot, "test-2-generic"), 0o755); err != nil {
			return err
		}
		targetDir := filepath.Join(modulesRoot, "test-1-generic")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(targetDir, "module.ko"), []byte("module"), 0o644)
	})

	destDir := filepath.Join(t.TempDir(), "kernel_modules")
	extractor := NewExtractor(config.KernelPackage{KernelRelease: "test-1-generic"}, workDir)
	if err := extractor.extractModules(filepath.Join(workDir, "modules.deb"), destDir); err != nil {
		t.Fatalf("extractModules() error = %v", err)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "module.ko"))
	if err != nil {
		t.Fatalf("read module: %v", err)
	}
	if string(got) != "module" {
		t.Fatalf("module content = %q, want %q", string(got), "module")
	}
}

func TestExtractModulesFallsBackToMergedUsrPath(t *testing.T) {
	workDir := t.TempDir()
	installFakeDpkgDeb(t, func(outDir string) error {
		modulesRoot := filepath.Join(outDir, "usr", "lib", "modules")
		targetDir := filepath.Join(modulesRoot, "test-1-generic")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(targetDir, "module.ko"), []byte("module"), 0o644)
	})

	destDir := filepath.Join(t.TempDir(), "kernel_modules")
	extractor := NewExtractor(config.KernelPackage{KernelRelease: "test-1-generic"}, workDir)
	if err := extractor.extractModules(filepath.Join(workDir, "modules.deb"), destDir); err != nil {
		t.Fatalf("extractModules() error = %v", err)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "module.ko"))
	if err != nil {
		t.Fatalf("read module: %v", err)
	}
	if string(got) != "module" {
		t.Fatalf("module content = %q, want %q", string(got), "module")
	}
}

func TestExtractDebPackageWrapsCommandError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test requires a POSIX shell")
	}
	binDir := t.TempDir()
	mustWriteFile(t, filepath.Join(binDir, "dpkg-deb"), []byte("#!/bin/sh\nprintf 'fake failure\\n' >&2\nexit 7\n"), 0o755)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := extractDebPackage("package.deb", t.TempDir())
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("errors.As(%v, *exec.ExitError) = false", err)
	}
	if got, want := err.Error(), fmt.Sprintf("dpkg-deb failed: %v, output: %s", exitErr, "fake failure\n"); got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func installFakeDpkgDeb(t *testing.T, populate func(outDir string) error) {
	t.Helper()

	binDir := t.TempDir()
	callsDir := t.TempDir()
	scriptPath := filepath.Join(binDir, "dpkg-deb")
	populateScript := filepath.Join(callsDir, "populate.sh")

	mustWriteFile(t, populateScript, []byte(fakePopulateScript(t, populate)), 0o755)
	mustWriteFile(t, scriptPath, []byte(fmt.Sprintf("#!/usr/bin/env sh\nset -eu\n\"%s\" \"$3\"\n", populateScript)), 0o755)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func fakePopulateScript(t *testing.T, populate func(outDir string) error) string {
	t.Helper()

	markerDir := t.TempDir()
	if err := populate(markerDir); err != nil {
		t.Fatalf("prepare fake dpkg-deb content: %v", err)
	}

	if runtime.GOOS == "windows" {
		t.Fatal("this test requires a POSIX shell; Windows is not supported")
	}

	return fmt.Sprintf(`#!/usr/bin/env sh
set -eu
cp -R %s/. "$1"
`, shellQuote(markerDir))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
