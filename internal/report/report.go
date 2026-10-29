package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/verirelay/protocol/guest"
	"github.com/verirelay/veriforge/internal/config"
	"github.com/verirelay/veriforge/internal/forge"
	"github.com/verirelay/veriforge/internal/manifest"
)

const Version = "verirelay-build-report/1"

type BuildReport struct {
	Version         string          `json:"version"`
	Manifest        guest.Manifest  `json:"manifest"`
	VeriRelayBinary FileDigest      `json:"verirelay_binary"`
	VeriForge       VeriForgeState  `json:"veriforge"`
	Target          TargetInfo      `json:"target"`
	Toolchain       ToolchainInfo   `json:"toolchain"`
	Kernel          KernelInfo      `json:"kernel"`
	SourceDateEpoch int64           `json:"source_date_epoch"`
	CPIO            CPIOInfo        `json:"cpio"`
	Compression     CompressionInfo `json:"compression"`
	Artifacts       ArtifactDigests `json:"artifacts"`
}

type FileDigest struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type VeriForgeState struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	Dirty      bool   `json:"dirty"`
}

type TargetInfo struct {
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	KernelRelease string `json:"kernel_release"`
}

type ToolchainInfo struct {
	GoVersion string `json:"go_version"`
}

type KernelInfo struct {
	KernelURL     string `json:"kernel_url"`
	KernelSHA256  string `json:"kernel_sha256"`
	ModulesURL    string `json:"modules_url"`
	ModulesSHA256 string `json:"modules_sha256"`
}

type CPIOInfo struct {
	Format string `json:"format"`
	UID    int    `json:"uid"`
	GID    int    `json:"gid"`
	MTime  int64  `json:"mtime"`
}

type CompressionInfo struct {
	Algorithm string `json:"algorithm"`
	Level     string `json:"level"`
}

type ArtifactDigests struct {
	Manifest FileDigest `json:"manifest"`
	Binary   FileDigest `json:"binary"`
	Kernel   FileDigest `json:"kernel"`
	Initrd   FileDigest `json:"initrd"`
}

type Inputs struct {
	Manifest      guest.Manifest
	ManifestBytes []byte
	VeriForge     manifest.GitIdentity
	Kernel        config.KernelPackage
	BinaryPath    string
	KernelPath    string
	InitrdPath    string
}

func New(inputs Inputs) (BuildReport, []byte, error) {
	manifestDigest := DigestBytes(inputs.ManifestBytes)
	binaryDigest, err := DigestFile(inputs.BinaryPath)
	if err != nil {
		return BuildReport{}, nil, fmt.Errorf("hash VeriRelay binary: %w", err)
	}
	kernelDigest, err := DigestFile(inputs.KernelPath)
	if err != nil {
		return BuildReport{}, nil, fmt.Errorf("hash kernel: %w", err)
	}
	initrdDigest, err := DigestFile(inputs.InitrdPath)
	if err != nil {
		return BuildReport{}, nil, fmt.Errorf("hash initrd: %w", err)
	}

	report := BuildReport{
		Version:         Version,
		Manifest:        inputs.Manifest,
		VeriRelayBinary: binaryDigest,
		VeriForge: VeriForgeState{
			Repository: inputs.VeriForge.Repository,
			Commit:     inputs.VeriForge.Commit,
			Dirty:      inputs.VeriForge.Dirty,
		},
		Target: TargetInfo{
			OS:            "linux",
			Arch:          "amd64",
			KernelRelease: inputs.Kernel.KernelRelease,
		},
		Toolchain: ToolchainInfo{GoVersion: runtime.Version()},
		Kernel: KernelInfo{
			KernelURL:     inputs.Kernel.KernelURL,
			KernelSHA256:  inputs.Kernel.KernelHash,
			ModulesURL:    inputs.Kernel.ModulesURL,
			ModulesSHA256: inputs.Kernel.ModulesHash,
		},
		SourceDateEpoch: forge.DeterministicMTime,
		CPIO: CPIOInfo{
			Format: forge.CPIOFormat,
			UID:    forge.DeterministicUID,
			GID:    forge.DeterministicGID,
			MTime:  forge.DeterministicMTime,
		},
		Compression: CompressionInfo{
			Algorithm: forge.CompressionAlgorithm,
			Level:     forge.CompressionLevel,
		},
		Artifacts: ArtifactDigests{
			Manifest: manifestDigest,
			Binary:   binaryDigest,
			Kernel:   kernelDigest,
			Initrd:   initrdDigest,
		},
	}

	data, err := Marshal(report)
	if err != nil {
		return BuildReport{}, nil, err
	}
	return report, data, nil
}

func DigestBytes(data []byte) FileDigest {
	sum := sha256.Sum256(data)
	return FileDigest{SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data))}
}

func DigestFile(path string) (FileDigest, error) {
	f, err := os.Open(path)
	if err != nil {
		return FileDigest{}, err
	}
	defer func() {
		_ = f.Close()
	}()

	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return FileDigest{}, err
	}
	return FileDigest{SHA256: hex.EncodeToString(h.Sum(nil)), Size: n}, nil
}

func Marshal(report BuildReport) ([]byte, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal build report: %w", err)
	}
	return append(data, '\n'), nil
}

func WriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create build report output directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write build report: %w", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		return fmt.Errorf("set build report permissions: %w", err)
	}
	return nil
}
