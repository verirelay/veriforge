package forge

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	caBundleAssetPath = "cacerts/ca-certificates.crt"
	caHashAssetPath   = "cacerts/ca-certificates.crt.sha256"
)

//go:embed cacerts/ca-certificates.crt cacerts/ca-certificates.crt.sha256
var caCertsFS embed.FS

// InstallCACertificates verifies the embedded CA bundle and installs it at the standard rootfs certificate paths.
func InstallCACertificates(rootfsDir string) error {
	bundle, err := caCertsFS.ReadFile(caBundleAssetPath)
	if err != nil {
		return fmt.Errorf("read embedded CA bundle: %w", err)
	}
	expectedHash, err := embeddedCABundleHash()
	if err != nil {
		return err
	}

	sum := sha256.Sum256(bundle)
	actualHash := hex.EncodeToString(sum[:])

	if actualHash != expectedHash {
		return fmt.Errorf("CA bundle SHA256 verification failed: expected %s, got %s", expectedHash, actualHash)
	}

	certsDir := filepath.Join(rootfsDir, "etc", "ssl", "certs")
	if err := os.MkdirAll(certsDir, 0o755); err != nil {
		return fmt.Errorf("create CA certificates directory: %w", err)
	}
	bundlePath := filepath.Join(certsDir, "ca-certificates.crt")
	if err := os.WriteFile(bundlePath, bundle, 0o644); err != nil {
		return fmt.Errorf("write CA bundle: %w", err)
	}

	certPEMPath := filepath.Join(rootfsDir, "etc", "ssl", "cert.pem")
	if err := os.Remove(certPEMPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove old cert.pem: %w", err)
	}
	if err := os.Symlink(filepath.Join("certs", "ca-certificates.crt"), certPEMPath); err != nil {
		return fmt.Errorf("create cert.pem symlink: %w", err)
	}

	fmt.Println("CA certificates installed")
	return nil
}

func embeddedCABundleHash() (string, error) {
	raw, err := caCertsFS.ReadFile(caHashAssetPath)
	if err != nil {
		return "", fmt.Errorf("read CA bundle hash: %w", err)
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return "", errors.New("CA bundle hash file is empty")
	}
	expectedHash := strings.ToLower(fields[0])
	decoded, err := hex.DecodeString(expectedHash)
	if err != nil || len(decoded) != sha256.Size {
		return "", fmt.Errorf("invalid CA bundle hash format: %q", fields[0])
	}
	return expectedHash, nil
}
