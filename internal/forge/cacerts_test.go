package forge

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallCACertificatesWritesBundleAndCompatSymlink(t *testing.T) {
	rootfs := t.TempDir()

	if err := InstallCACertificates(rootfs); err != nil {
		t.Fatalf("InstallCACertificates() error = %v", err)
	}

	bundlePath := filepath.Join(rootfs, "etc", "ssl", "certs", "ca-certificates.crt")
	bundle, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("read rootfs CA bundle: %v", err)
	}
	if len(bundle) == 0 {
		t.Fatal("rootfs CA bundle is empty")
	}

	bundleStat, err := os.Stat(bundlePath)
	if err != nil {
		t.Fatalf("stat rootfs CA bundle: %v", err)
	}
	if bundleStat.Mode().Perm() != 0o644 {
		t.Fatalf("rootfs CA bundle mode = %#o, want %#o", bundleStat.Mode().Perm(), os.FileMode(0o644))
	}

	linkPath := filepath.Join(rootfs, "etc", "ssl", "cert.pem")
	linkTarget, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("read cert.pem symlink: %v", err)
	}
	if linkTarget != filepath.Join("certs", "ca-certificates.crt") {
		t.Fatalf("cert.pem symlink = %q, want %q", linkTarget, filepath.Join("certs", "ca-certificates.crt"))
	}
}

func TestEmbeddedCABundleHashMatchesBundle(t *testing.T) {
	bundle, err := caCertsFS.ReadFile(caBundleAssetPath)
	if err != nil {
		t.Fatalf("read embedded CA bundle: %v", err)
	}
	expectedHash, err := embeddedCABundleHash()
	if err != nil {
		t.Fatalf("embeddedCABundleHash() error = %v", err)
	}
	sum := sha256.Sum256(bundle)
	if actualHash := hex.EncodeToString(sum[:]); actualHash != expectedHash {
		t.Fatalf("CA bundle hash = %s, want %s", actualHash, expectedHash)
	}
}
