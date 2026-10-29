package manifest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/verirelay/protocol/guest"
	"github.com/verirelay/veriforge/internal/forge"
)

var Path = strings.TrimPrefix(guest.ManifestPath, "/")

type Inputs struct {
	VeriRelayRepository string
	VeriRelayCommit     string
	VCPUType            string
	KernelAppend        string
	RepoDir             string
}

type GitIdentity struct {
	Repository string
	Commit     string
	Dirty      bool
}

func New(inputs Inputs) (guest.Manifest, GitIdentity, []byte, error) {
	veriforge, err := ResolveVeriForgeGitIdentity(inputs.RepoDir)
	if err != nil {
		return guest.Manifest{}, GitIdentity{}, nil, err
	}

	m := guest.Manifest{
		Version: guest.ManifestVersion,
		Source: guest.Source{
			VeriRelay: guest.SourceRef{Repository: inputs.VeriRelayRepository, Commit: inputs.VeriRelayCommit},
			VeriForge: guest.SourceRef{Repository: veriforge.Repository, Commit: veriforge.Commit},
		},
		LaunchProfile: guest.LaunchProfile{
			Version:            guest.LaunchProfileVersion,
			MeasurementProfile: guest.MeasurementProfile,
			VCPUType:           inputs.VCPUType,
			GuestFeatures:      guest.GuestFeatures,
			KernelHashes:       guest.KernelHashes,
			KernelAppend:       inputs.KernelAppend,
		},
	}
	if err := m.Validate(); err != nil {
		return guest.Manifest{}, GitIdentity{}, nil, fmt.Errorf("invalid guest manifest: %w", err)
	}

	data, err := Marshal(m)
	if err != nil {
		return guest.Manifest{}, GitIdentity{}, nil, err
	}
	return m, veriforge, data, nil
}

func ResolveVeriForgeGitIdentity(repoDir string) (GitIdentity, error) {
	topLevel, err := gitOutput(repoDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return GitIdentity{}, fmt.Errorf("resolve VeriForge git worktree: %w", err)
	}

	repository, err := gitOutput(topLevel, "remote", "get-url", "origin")
	if err != nil {
		return GitIdentity{}, fmt.Errorf("resolve VeriForge origin repository: %w", err)
	}
	if err := guest.ValidateRepositoryURL(repository); err != nil {
		return GitIdentity{}, fmt.Errorf("invalid VeriForge origin repository %q: %w", repository, err)
	}

	commit, err := gitOutput(topLevel, "rev-parse", "HEAD")
	if err != nil {
		return GitIdentity{}, fmt.Errorf("resolve VeriForge HEAD commit: %w", err)
	}

	dirty, err := gitOutput(topLevel, "status", "--porcelain")
	if err != nil {
		return GitIdentity{}, fmt.Errorf("check VeriForge dirty state: %w", err)
	}

	return GitIdentity{Repository: repository, Commit: commit, Dirty: dirty != ""}, nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func Marshal(m guest.Manifest) ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	return append(data, '\n'), nil
}

func WriteFile(rootfsDir string, data []byte) error {
	path := filepath.Join(rootfsDir, Path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		return fmt.Errorf("set manifest permissions: %w", err)
	}
	return nil
}

func VerifyInInitrd(initrdPath string, expected guest.Manifest, forbiddenSubstrings ...string) error {
	entries, err := forge.ReadInitrdEntries(initrdPath)
	if err != nil {
		return err
	}

	var manifestEntry *forge.InitrdEntry
	for i := range entries {
		if entries[i].Name == "./"+Path {
			manifestEntry = &entries[i]
			break
		}
	}
	if manifestEntry == nil {
		return fmt.Errorf("initrd missing ./%s", Path)
	}
	if manifestEntry.Mode != 0o100644 {
		return fmt.Errorf("manifest mode = %#o, want %#o", manifestEntry.Mode, int64(0o100644))
	}
	if manifestEntry.UID != 0 || manifestEntry.GID != 0 || manifestEntry.MTime != 0 {
		return fmt.Errorf("manifest metadata = uid %d gid %d mtime %d, want all zero", manifestEntry.UID, manifestEntry.GID, manifestEntry.MTime)
	}
	if !bytes.HasSuffix(manifestEntry.Data, []byte("\n")) || bytes.HasSuffix(manifestEntry.Data, []byte("\n\n")) {
		return errors.New("manifest must end with exactly one newline")
	}

	for _, forbidden := range forbiddenSubstrings {
		if forbidden == "" {
			continue
		}
		if bytes.Contains(manifestEntry.Data, []byte(forbidden)) {
			return fmt.Errorf("manifest contains forbidden local path %q", forbidden)
		}
	}

	actual, err := guest.DecodeManifest(manifestEntry.Data)
	if err != nil {
		return err
	}
	if actual != expected {
		return errors.New("manifest mismatch")
	}
	return nil
}
