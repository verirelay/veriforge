package forge

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDeterministicArchiverProducesStableBytes(t *testing.T) {
	rootfs := makeTestRootfs(t)
	first := filepath.Join(t.TempDir(), "first.img")
	second := filepath.Join(t.TempDir(), "second.img")

	if err := NewDeterministicArchiver(rootfs).BuildInitrd(first); err != nil {
		t.Fatalf("build first initrd: %v", err)
	}
	if err := NewDeterministicArchiver(rootfs).BuildInitrd(second); err != nil {
		t.Fatalf("build second initrd: %v", err)
	}

	firstBytes, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read first initrd: %v", err)
	}
	secondBytes, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("read second initrd: %v", err)
	}

	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("two archives of the same rootfs differ")
	}
}

func TestDeterministicArchiverWritesNewcEntries(t *testing.T) {
	rootfs := makeTestRootfs(t)
	initrd := filepath.Join(t.TempDir(), "nested", "initrd.img")

	if err := NewDeterministicArchiver(rootfs).BuildInitrd(initrd); err != nil {
		t.Fatalf("build initrd: %v", err)
	}

	entries := readInitrdEntries(t, initrd)
	byName := make(map[string]InitrdEntry, len(entries))
	for _, entry := range entries {
		byName[entry.Name] = entry
	}

	initEntry, ok := byName["./init"]
	if !ok {
		t.Fatal("CPIO is missing ./init")
	}
	if initEntry.Mode != 0o100755 {
		t.Fatalf("./init mode = %#o, want %#o", initEntry.Mode, int64(0o100755))
	}
	if string(initEntry.Data) != "#!/bin/sh\necho ok\n" {
		t.Fatalf("./init content = %q", string(initEntry.Data))
	}

	moduleEntry, ok := byName["./modules/virtio_net.ko"]
	if !ok {
		t.Fatal("CPIO is missing ./modules/virtio_net.ko")
	}
	if moduleEntry.Mode != 0o100755 {
		t.Fatalf("module mode = %#o, want %#o", moduleEntry.Mode, int64(0o100755))
	}

	consoleEntry, ok := byName["./dev/console"]
	if !ok {
		t.Fatal("CPIO is missing ./dev/console")
	}
	if consoleEntry.Mode != 0o020600 || consoleEntry.RdevMajor != 5 || consoleEntry.RdevMinor != 1 {
		t.Fatalf("/dev/console = mode %#o rdev %d:%d, want mode %#o rdev 5:1", consoleEntry.Mode, consoleEntry.RdevMajor, consoleEntry.RdevMinor, int64(0o020600))
	}

	symlinkEntry, ok := byName["./bin/sh"]
	if !ok {
		t.Fatal("CPIO is missing ./bin/sh")
	}
	if symlinkEntry.Mode != 0o120777 {
		t.Fatalf("symlink mode = %#o, want %#o", symlinkEntry.Mode, int64(0o120777))
	}
	if string(symlinkEntry.Data) != "busybox" {
		t.Fatalf("symlink payload = %q, want %q", string(symlinkEntry.Data), "busybox")
	}

	etcEntry, ok := byName["./etc/config"]
	if !ok {
		t.Fatal("CPIO is missing ./etc/config")
	}
	if etcEntry.Mode != 0o100644 {
		t.Fatalf("regular file mode = %#o, want %#o", etcEntry.Mode, int64(0o100644))
	}

	manifestEntry, ok := byName["./etc/verirelay/manifest.json"]
	if !ok {
		t.Fatal("CPIO is missing ./etc/verirelay/manifest.json")
	}
	if manifestEntry.Mode != 0o100644 {
		t.Fatalf("manifest.json mode = %#o, want %#o", manifestEntry.Mode, int64(0o100644))
	}
	if string(manifestEntry.Data) != "{\"ok\":true}\n" {
		t.Fatalf("manifest.json content = %q", string(manifestEntry.Data))
	}

	for _, entry := range entries {
		if entry.UID != 0 || entry.GID != 0 || entry.MTime != 0 {
			t.Fatalf("%s metadata not zeroed: uid=%d gid=%d mtime=%d", entry.Name, entry.UID, entry.GID, entry.MTime)
		}
	}

	if _, ok := byName["TRAILER!!!"]; !ok {
		t.Fatal("CPIO is missing TRAILER!!!")
	}
}

func makeTestRootfs(t *testing.T) string {
	t.Helper()

	rootfs := t.TempDir()
	mustMkdirAll(t, filepath.Join(rootfs, "bin"))
	mustMkdirAll(t, filepath.Join(rootfs, "dev"))
	mustMkdirAll(t, filepath.Join(rootfs, "modules"))
	mustMkdirAll(t, filepath.Join(rootfs, "etc", "verirelay"))
	mustWriteFile(t, filepath.Join(rootfs, "init"), []byte("#!/bin/sh\necho ok\n"), 0o755)
	mustWriteFile(t, filepath.Join(rootfs, "dev", "console"), nil, 0o600)
	mustWriteFile(t, filepath.Join(rootfs, "modules", "virtio_net.ko"), []byte{0xde, 0xad, 0xbe, 0xef}, 0o644)
	mustWriteFile(t, filepath.Join(rootfs, "etc", "config"), []byte("CONFIG=1\n"), 0o644)
	mustWriteFile(t, filepath.Join(rootfs, "etc", "verirelay", "manifest.json"), []byte("{\"ok\":true}\n"), 0o644)
	if err := os.Symlink("busybox", filepath.Join(rootfs, "bin", "sh")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	return rootfs
}

func readInitrdEntries(t *testing.T, initrd string) []InitrdEntry {
	t.Helper()

	entries, err := ReadInitrdEntries(initrd)
	if err != nil {
		t.Fatalf("read initrd entries: %v", err)
	}
	return entries
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("create directory %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
