package forge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/verirelay/veriforge/internal/config"
)

func TestIndexModulesMatchesSupportedKernelObjectNames(t *testing.T) {
	baseDir := t.TempDir()
	mustMkdirAll(t, filepath.Join(baseDir, "kernel", "drivers", "net"))
	mustMkdirAll(t, filepath.Join(baseDir, "kernel", "arch", "x86"))
	mustMkdirAll(t, filepath.Join(baseDir, "kernel", "crypto"))
	mustWriteFile(t, filepath.Join(baseDir, "kernel", "drivers", "net", "virtio_net.ko"), []byte("plain"), 0o644)
	mustWriteFile(t, filepath.Join(baseDir, "kernel", "arch", "x86", "sev-guest.ko.zst"), []byte("zstd"), 0o644)
	mustWriteFile(t, filepath.Join(baseDir, "kernel", "crypto", "ccp.ko.xz"), []byte("xz"), 0o644)

	moduleIndex, err := NewModulePruner(baseDir, t.TempDir(), config.KernelPackage{}).indexModules()
	if err != nil {
		t.Fatalf("indexModules() error = %v", err)
	}
	tests := []struct {
		name       string
		module     string
		wantSuffix string
	}{
		{name: "plain ko", module: "virtio_net", wantSuffix: filepath.Join("kernel", "drivers", "net", "virtio_net.ko")},
		{name: "zstd compressed ko", module: "sev-guest", wantSuffix: filepath.Join("kernel", "arch", "x86", "sev-guest.ko.zst")},
		{name: "xz compressed ko", module: "ccp", wantSuffix: filepath.Join("kernel", "crypto", "ccp.ko.xz")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate, ok := moduleIndex[tt.module]
			if !ok {
				t.Fatalf("indexModules() is missing %s", tt.module)
			}
			if !strings.HasSuffix(candidate.sourcePath, tt.wantSuffix) {
				t.Fatalf("indexModules()[%s] = %s, want suffix %s", tt.module, candidate.sourcePath, tt.wantSuffix)
			}
		})
	}
}

func TestPruneAndBuildRootfsCopiesInitAndWritesFoundModules(t *testing.T) {
	modulesDir := t.TempDir()
	rootfsDir := t.TempDir()
	initBin := filepath.Join(t.TempDir(), "init")
	mustWriteFile(t, initBin, []byte("init"), 0o755)
	mustWriteFile(t, filepath.Join(modulesDir, "dependency.ko"), moduleWithDepends("dependency", ""), 0o644)
	mustWriteFile(t, filepath.Join(modulesDir, "required.ko"), moduleWithDepends("required", "dependency"), 0o644)
	mustWriteFile(t, filepath.Join(modulesDir, "modules.builtin"), []byte("kernel/drivers/block/virtio_blk.ko\n"), 0o644)

	pruner := NewModulePruner(modulesDir, rootfsDir, config.KernelPackage{
		KernelRelease:   "test-release",
		RequiredModules: []string{"required", "virtio_blk"},
	})

	if err := pruner.PruneAndBuildRootfs(initBin); err != nil {
		t.Fatalf("PruneAndBuildRootfs() error = %v", err)
	}

	initStat, err := os.Stat(filepath.Join(rootfsDir, "init"))
	if err != nil {
		t.Fatalf("rootfs init missing: %v", err)
	}
	if initStat.Mode().Perm() != 0o755 {
		t.Fatalf("rootfs init mode = %#o, want %#o", initStat.Mode().Perm(), os.FileMode(0o755))
	}

	consoleStat, err := os.Stat(filepath.Join(rootfsDir, "dev", "console"))
	if err != nil {
		t.Fatalf("rootfs /dev/console placeholder missing: %v", err)
	}
	if consoleStat.Mode().Perm() != 0o600 {
		t.Fatalf("rootfs /dev/console mode = %#o, want %#o", consoleStat.Mode().Perm(), os.FileMode(0o600))
	}

	modulesRoot := filepath.Join(rootfsDir, "lib", "modules", "test-release")
	assertFileContent(t, filepath.Join(modulesRoot, "dependency.ko"), string(moduleWithDepends("dependency", "")))
	assertFileContent(t, filepath.Join(modulesRoot, "required.ko"), string(moduleWithDepends("required", "dependency")))

	orderBytes, err := os.ReadFile(filepath.Join(modulesRoot, "modules.order"))
	if err != nil {
		t.Fatalf("read modules.order: %v", err)
	}
	if string(orderBytes) != "dependency.ko\nrequired.ko\n" {
		t.Fatalf("modules.order = %q, want %q", string(orderBytes), "dependency.ko\nrequired.ko\n")
	}
}

func TestPruneAndBuildRootfsFailsWhenRequiredModuleIsMissing(t *testing.T) {
	modulesDir := t.TempDir()
	rootfsDir := t.TempDir()
	initBin := filepath.Join(t.TempDir(), "init")
	mustWriteFile(t, initBin, []byte("init"), 0o755)
	mustWriteFile(t, filepath.Join(modulesDir, "other.ko"), moduleWithDepends("other", ""), 0o644)
	mustWriteFile(t, filepath.Join(modulesDir, "modules.builtin"), []byte("kernel/drivers/block/virtio_blk.ko\n"), 0o644)

	pruner := NewModulePruner(modulesDir, rootfsDir, config.KernelPackage{
		KernelRelease:   "test-release",
		RequiredModules: []string{"missing_required"},
	})

	err := pruner.PruneAndBuildRootfs(initBin)
	if err == nil || !strings.Contains(err.Error(), "neither a module nor built into") {
		t.Fatalf("PruneAndBuildRootfs() error = %v, want required module missing error", err)
	}
}

func moduleWithDepends(name, depends string) []byte {
	return []byte("name=" + name + "\x00depends=" + depends + "\x00")
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s content = %q, want %q", path, string(got), want)
	}
}
