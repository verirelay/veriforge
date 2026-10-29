package report

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/verirelay/protocol/guest"
	"github.com/verirelay/veriforge/internal/config"
	"github.com/verirelay/veriforge/internal/manifest"
)

func TestDigestBytesAndFile(t *testing.T) {
	dataDigest := DigestBytes([]byte("hello\n"))
	if dataDigest.SHA256 != "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03" {
		t.Fatalf("sha256 = %s", dataDigest.SHA256)
	}
	if dataDigest.Size != 6 {
		t.Fatalf("size = %d, want 6", dataDigest.Size)
	}

	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	fileDigest, err := DigestFile(path)
	if err != nil {
		t.Fatalf("DigestFile returned error: %v", err)
	}
	if fileDigest != dataDigest {
		t.Fatalf("DigestFile = %+v, want %+v", fileDigest, dataDigest)
	}
}

func TestNewBuildReport(t *testing.T) {
	m := testManifest()
	manifestBytes, err := manifest.Marshal(m)
	if err != nil {
		t.Fatalf("manifest Marshal returned error: %v", err)
	}
	dir := t.TempDir()
	binaryPath := writeTestFile(t, dir, "verirelay", []byte("binary\n"))
	kernelPath := writeTestFile(t, dir, "vmlinuz", []byte("kernel\n"))
	initrdPath := writeTestFile(t, dir, "initrd.img", []byte("initrd\n"))

	got, data, err := New(Inputs{
		Manifest:      m,
		ManifestBytes: manifestBytes,
		VeriForge: manifest.GitIdentity{
			Repository: "https://github.com/verirelay/veriforge",
			Commit:     "89abcdef0123456789abcdef0123456789abcdef",
			Dirty:      true,
		},
		Kernel:     config.DefaultKernel,
		BinaryPath: binaryPath,
		KernelPath: kernelPath,
		InitrdPath: initrdPath,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if got.Version != Version {
		t.Fatalf("version = %q", got.Version)
	}
	if got.Manifest != m {
		t.Fatal("report manifest does not match input manifest")
	}
	if !got.VeriForge.Dirty {
		t.Fatal("dirty state not recorded")
	}
	if got.Artifacts.Manifest != DigestBytes(manifestBytes) {
		t.Fatalf("manifest digest = %+v", got.Artifacts.Manifest)
	}
	if got.VeriRelayBinary != got.Artifacts.Binary {
		t.Fatal("verirelay_binary does not match artifacts.binary")
	}
	if bytes.Contains(data, []byte(binaryPath)) || bytes.Contains(data, []byte(dir)) {
		t.Fatal("build report contains local path")
	}
	for _, forbidden := range []string{"report_sha256", "OVMF", "firmware"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("build report contains forbidden substring %q", forbidden)
		}
	}

	_, again, err := New(Inputs{
		Manifest:      m,
		ManifestBytes: manifestBytes,
		VeriForge: manifest.GitIdentity{
			Repository: "https://github.com/verirelay/veriforge",
			Commit:     "89abcdef0123456789abcdef0123456789abcdef",
			Dirty:      true,
		},
		Kernel:     config.DefaultKernel,
		BinaryPath: binaryPath,
		KernelPath: kernelPath,
		InitrdPath: initrdPath,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if !bytes.Equal(data, again) {
		t.Fatal("same inputs produced different build report bytes")
	}
}

func TestWriteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "build-report.json")
	data := []byte("{}\n")
	if err := WriteFile(path, data); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("report = %q, want %q", string(got), string(data))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat report: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("report mode = %#o", info.Mode().Perm())
	}
}

func testManifest() guest.Manifest {
	return guest.Manifest{
		Version: guest.ManifestVersion,
		Source: guest.Source{
			VeriRelay: guest.SourceRef{Repository: "https://github.com/verirelay/verirelay", Commit: "0123456789abcdef0123456789abcdef01234567"},
			VeriForge: guest.SourceRef{Repository: "https://github.com/verirelay/veriforge", Commit: "89abcdef0123456789abcdef0123456789abcdef"},
		},
		LaunchProfile: guest.LaunchProfile{
			Version:            guest.LaunchProfileVersion,
			MeasurementProfile: guest.MeasurementProfile,
			VCPUType:           guest.VCPUType,
			GuestFeatures:      guest.GuestFeatures,
			KernelHashes:       guest.KernelHashes,
			KernelAppend:       "console=ttyS0 panic=1",
		},
	}
}

func writeTestFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}
