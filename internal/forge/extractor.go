package forge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/verirelay/veriforge/internal/config"
)

// Extractor downloads, verifies, and unpacks the kernel and modules packages.
type Extractor struct {
	cfg        config.KernelPackage
	workDir    string
	httpClient *http.Client
}

func NewExtractor(cfg config.KernelPackage, workDir string) *Extractor {
	return &Extractor{
		cfg:        cfg,
		workDir:    workDir,
		httpClient: &http.Client{Timeout: 10 * time.Minute},
	}
}

// PrepareArtifacts produces the vmlinuz and the full modules directory needed to build the initrd.
func (e *Extractor) PrepareArtifacts(ctx context.Context, outDir string) (string, string, error) {
	if err := os.MkdirAll(e.workDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create cache directory: %w", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create output directory: %w", err)
	}

	kernelDeb := filepath.Join(e.workDir, "kernel.deb")
	modulesDeb := filepath.Join(e.workDir, "modules.deb")

	fmt.Println("Preparing kernel package")
	if err := e.downloadAndVerify(ctx, e.cfg.KernelURL, kernelDeb, e.cfg.KernelHash); err != nil {
		return "", "", err
	}

	fmt.Println("Preparing modules package")
	if err := e.downloadAndVerify(ctx, e.cfg.ModulesURL, modulesDeb, e.cfg.ModulesHash); err != nil {
		return "", "", err
	}

	vmlinuzPath := filepath.Join(outDir, "vmlinuz")
	if err := e.extractVmlinuz(kernelDeb, vmlinuzPath); err != nil {
		return "", "", fmt.Errorf("extract vmlinuz: %w", err)
	}

	modulesExtractDir := filepath.Join(outDir, "kernel_modules")
	if err := e.extractModules(modulesDeb, modulesExtractDir); err != nil {
		return "", "", fmt.Errorf("extract modules: %w", err)
	}

	return vmlinuzPath, modulesExtractDir, nil
}

func (e *Extractor) downloadAndVerify(ctx context.Context, url, targetPath, expectedHash string) error {
	if fileHasSHA256(targetPath, expectedHash) {
		fmt.Printf("cache hit: %s\n", filepath.Base(targetPath))
		return nil
	}

	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}

	abort := func(err error) error {
		_ = out.Close()
		_ = os.Remove(targetPath)
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return abort(fmt.Errorf("download failed: %w", err))
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return abort(fmt.Errorf("download failed: %w", err))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return abort(fmt.Errorf("server returned unexpected status code: %d", resp.StatusCode))
	}

	hasher := sha256.New()
	_, copyErr := io.Copy(out, io.TeeReader(resp.Body, hasher))
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(targetPath)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(targetPath)
		return closeErr
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != expectedHash {
		if err := os.Remove(targetPath); err != nil {
			return fmt.Errorf("remove invalid downloaded file: %w", err)
		}
		return fmt.Errorf("SHA256 verification failed: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

func fileHasSHA256(path, expectedHash string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() {
		_ = f.Close()
	}()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false
	}
	return hex.EncodeToString(h.Sum(nil)) == expectedHash
}

func (e *Extractor) extractVmlinuz(debPath, destPath string) error {
	tmpDir, err := os.MkdirTemp(e.workDir, "kernel_unpack_*")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	if err := extractDebPackage(debPath, tmpDir); err != nil {
		return err
	}

	srcVmlinuz := filepath.Join(tmpDir, "boot", "vmlinuz-"+e.cfg.KernelRelease)
	if _, err := os.Stat(srcVmlinuz); err != nil {
		return fmt.Errorf("vmlinuz for kernel release %q not found: %w", e.cfg.KernelRelease, err)
	}
	return CopyFile(srcVmlinuz, destPath)
}

func (e *Extractor) extractModules(debPath, destDir string) error {
	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("clean old modules directory: %w", err)
	}

	tmpDir, err := os.MkdirTemp(e.workDir, "modules_unpack_*")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	if err := extractDebPackage(debPath, tmpDir); err != nil {
		return err
	}

	srcModulesPath := filepath.Join(tmpDir, "lib", "modules", e.cfg.KernelRelease)
	if _, err := os.Stat(srcModulesPath); os.IsNotExist(err) {
		srcModulesPath = filepath.Join(tmpDir, "usr", "lib", "modules", e.cfg.KernelRelease)
	}
	if _, err := os.Stat(srcModulesPath); err != nil {
		return fmt.Errorf("modules directory for kernel release %q not found: %w", e.cfg.KernelRelease, err)
	}
	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		return err
	}
	return moveDir(srcModulesPath, destDir)
}

func extractDebPackage(debPath, destDir string) error {
	cmd := exec.CommandContext(context.Background(), "dpkg-deb", "-x", debPath, destDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("dpkg-deb failed: %w, output: %s", err, string(output))
	}
	return nil
}

// CopyFile copies src to dst, creating parent directories as needed; the partial target is removed on failure.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}

func moveDir(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	if err := copyDir(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		if err := CopyFile(path, targetPath); err != nil {
			return err
		}
		return os.Chmod(targetPath, info.Mode())
	})
}
