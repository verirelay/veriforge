package manifest

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/verirelay/protocol/guest"
	"github.com/verirelay/veriforge/internal/forge"
)

const validCommit = "0123456789abcdef0123456789abcdef01234567"

func TestManifestMarshalDeterministic(t *testing.T) {
	m := testManifest()
	data, err := Marshal(m)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	again, err := Marshal(m)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if !bytes.Equal(data, again) {
		t.Fatal("same manifest marshaled to different bytes")
	}
	want := "{\n  \"version\": \"verirelay-guest-manifest/1\",\n  \"source\": {\n    \"verirelay\": {\n      \"repository\": \"https://github.com/verirelay/verirelay\",\n      \"commit\": \"0123456789abcdef0123456789abcdef01234567\"\n    },\n    \"veriforge\": {\n      \"repository\": \"https://github.com/verirelay/veriforge\",\n      \"commit\": \"89abcdef0123456789abcdef0123456789abcdef\"\n    }\n  },\n  \"launch_profile\": {\n    \"version\": \"verirelay-launch-profile/1\",\n    \"measurement_profile\": \"sev-snp-qemu-direct-boot/1\",\n    \"vcpu_type\": \"EPYC-v4\",\n    \"guest_features\": \"0x1\",\n    \"kernel_hashes\": true,\n    \"kernel_append\": \"console=ttyS0 panic=1\"\n  }\n}\n"
	if string(data) != want {
		t.Fatalf("manifest JSON = %q, want %q", string(data), want)
	}
	changed := m
	changed.LaunchProfile.KernelAppend = "console=ttyS0"
	changedData, err := Marshal(changed)
	if err != nil {
		t.Fatalf("Marshal changed manifest returned error: %v", err)
	}
	if bytes.Equal(data, changedData) {
		t.Fatal("changing launch profile did not change manifest bytes")
	}
}

func TestNew(t *testing.T) {
	requireGit(t)
	repo := initTestGitRepo(t)
	identity, err := ResolveVeriForgeGitIdentity(repo)
	if err != nil {
		t.Fatalf("ResolveVeriForgeGitIdentity returned error: %v", err)
	}

	m, gotIdentity, data, err := New(validInputs(repo))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	want := testManifest()
	want.Source.VeriForge.Commit = identity.Commit
	if m != want {
		t.Fatalf("manifest = %+v, want %+v", m, want)
	}
	if gotIdentity != identity {
		t.Fatalf("identity = %+v, want %+v", gotIdentity, identity)
	}
	wantData, err := Marshal(want)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if !bytes.Equal(data, wantData) {
		t.Fatalf("manifest bytes = %q, want %q", data, wantData)
	}
}

func TestNewRejectsInvalidManifest(t *testing.T) {
	requireGit(t)
	repo := initTestGitRepo(t)

	tests := map[string]func(*Inputs){
		"repository with .git":       func(in *Inputs) { in.VeriRelayRepository += ".git" },
		"uppercase commit":           func(in *Inputs) { in.VeriRelayCommit = strings.ToUpper(validCommit) },
		"vcpu type":                  func(in *Inputs) { in.VCPUType = "EPYC-v3" },
		"empty kernel append":        func(in *Inputs) { in.KernelAppend = "" },
		"kernel append with newline": func(in *Inputs) { in.KernelAppend = "console=ttyS0\npanic=1" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			inputs := validInputs(repo)
			mutate(&inputs)
			if _, _, _, err := New(inputs); err == nil {
				t.Fatal("New succeeded, want invalid manifest error")
			}
		})
	}
}

func TestWriteManifest(t *testing.T) {
	data, err := Marshal(testManifest())
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	rootfs := t.TempDir()

	if err := WriteFile(rootfs, data); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	path := filepath.Join(rootfs, "etc", "verirelay", "manifest.json")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("manifest.json = %q, want %q", string(got), string(data))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat manifest.json: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("manifest.json mode = %#o, want %#o", info.Mode().Perm(), os.FileMode(0o644))
	}
}

func TestVerifyInInitrd(t *testing.T) {
	m := testManifest()
	rootfs := t.TempDir()
	mustWriteTestFile(t, filepath.Join(rootfs, "init"), []byte("#!/bin/sh\n"), 0o755)
	mustWriteTestFile(t, filepath.Join(rootfs, "dev", "console"), nil, 0o600)
	data, err := Marshal(m)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := WriteFile(rootfs, data); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	initrd := buildTestInitrd(t, rootfs)

	if err := VerifyInInitrd(initrd, m, "/tmp/local/verirelay"); err != nil {
		t.Fatalf("VerifyInInitrd returned error: %v", err)
	}
}

func TestVerifyInInitrdRejectsForbiddenPath(t *testing.T) {
	rootfs := t.TempDir()
	mustWriteTestFile(t, filepath.Join(rootfs, "init"), []byte("#!/bin/sh\n"), 0o755)
	mustWriteTestFile(t, filepath.Join(rootfs, "dev", "console"), nil, 0o600)
	mustWriteTestFile(t, filepath.Join(rootfs, "etc", "verirelay", "manifest.json"), []byte("{\"path\":\"/tmp/local/verirelay\"}\n"), 0o644)
	initrd := buildTestInitrd(t, rootfs)

	if err := VerifyInInitrd(initrd, testManifest(), "/tmp/local/verirelay"); err == nil {
		t.Fatal("VerifyInInitrd succeeded, want forbidden path error")
	}
}

func TestVerifyInInitrdRejectsMissingFile(t *testing.T) {
	rootfs := t.TempDir()
	mustWriteTestFile(t, filepath.Join(rootfs, "init"), []byte("#!/bin/sh\n"), 0o755)
	initrd := buildTestInitrd(t, rootfs)

	if err := VerifyInInitrd(initrd, testManifest()); err == nil {
		t.Fatal("VerifyInInitrd succeeded, want missing file error")
	}
}

func TestResolveVeriForgeGitIdentity(t *testing.T) {
	requireGit(t)

	repo := initTestGitRepo(t)
	got, err := ResolveVeriForgeGitIdentity(repo)
	if err != nil {
		t.Fatalf("ResolveVeriForgeGitIdentity returned error: %v", err)
	}
	if got.Repository != "https://github.com/verirelay/veriforge" {
		t.Fatalf("repository = %q", got.Repository)
	}
	if err := guest.ValidateCommitSHA(got.Commit); err != nil {
		t.Fatalf("commit invalid: %v", err)
	}
	if got.Dirty {
		t.Fatal("clean repo reported dirty")
	}
}

func TestResolveVeriForgeGitIdentityReportsDirty(t *testing.T) {
	requireGit(t)

	repo := initTestGitRepo(t)
	mustWriteTestFile(t, filepath.Join(repo, "dirty.txt"), []byte("dirty\n"), 0o644)

	got, err := ResolveVeriForgeGitIdentity(repo)
	if err != nil {
		t.Fatalf("ResolveVeriForgeGitIdentity returned error: %v", err)
	}
	if !got.Dirty {
		t.Fatal("dirty repo was not reported dirty")
	}
}

func TestResolveVeriForgeGitIdentityRejectsNonCanonicalRemote(t *testing.T) {
	requireGit(t)

	for _, remote := range []string{
		"https://token@github.com/verirelay/veriforge",
		"https://github.com/verirelay/veriforge.git",
		"git@github.com:verirelay/veriforge.git",
	} {
		t.Run(remote, func(t *testing.T) {
			repo := initTestGitRepo(t)
			runGit(t, repo, "remote", "set-url", "origin", remote)

			if _, err := ResolveVeriForgeGitIdentity(repo); err == nil {
				t.Fatal("ResolveVeriForgeGitIdentity succeeded, want non-canonical remote error")
			}
		})
	}
}

func testManifest() guest.Manifest {
	return guest.Manifest{
		Version: guest.ManifestVersion,
		Source: guest.Source{
			VeriRelay: guest.SourceRef{Repository: "https://github.com/verirelay/verirelay", Commit: validCommit},
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

func validInputs(repoDir string) Inputs {
	return Inputs{
		VeriRelayRepository: "https://github.com/verirelay/verirelay",
		VeriRelayCommit:     validCommit,
		VCPUType:            guest.VCPUType,
		KernelAppend:        "console=ttyS0 panic=1",
		RepoDir:             repoDir,
	}
}

func buildTestInitrd(t *testing.T, rootfs string) string {
	t.Helper()
	initrd := filepath.Join(t.TempDir(), "initrd.img")
	if err := forge.NewDeterministicArchiver(rootfs).BuildInitrd(initrd); err != nil {
		t.Fatalf("BuildInitrd returned error: %v", err)
	}
	return initrd
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
}

func initTestGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWriteTestFile(t, filepath.Join(repo, "README.md"), []byte("test\n"), 0o644)
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")
	runGit(t, repo, "remote", "add", "origin", "https://github.com/verirelay/veriforge")
	return repo
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func mustWriteTestFile(t *testing.T, path string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory: %v", err)
	}
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
