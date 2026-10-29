package forge

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/verirelay/veriforge/internal/config"
)

// ModulePruner selects the kernel modules that go into the initrd and builds the minimal rootfs.
type ModulePruner struct {
	extractedModulesDir string
	rootfsDir           string
	cfg                 config.KernelPackage
}

type moduleCandidate struct {
	sourcePath string
	moduleRel  string
}

func NewModulePruner(extractedModulesDir, rootfsDir string, cfg config.KernelPackage) *ModulePruner {
	return &ModulePruner{
		extractedModulesDir: extractedModulesDir,
		rootfsDir:           rootfsDir,
		cfg:                 cfg,
	}
}

// PruneAndBuildRootfs copies the init binary, extracts the configured modules, and writes modules.order.
func (p *ModulePruner) PruneAndBuildRootfs(initBinPath string) error {
	for _, dir := range []string{"dev", "proc", "sys", "bin", "sbin", "etc"} {
		if err := os.MkdirAll(filepath.Join(p.rootfsDir, dir), 0o755); err != nil {
			return err
		}
	}
	if err := ensureFile(filepath.Join(p.rootfsDir, "dev", "console")); err != nil {
		return fmt.Errorf("create /dev/console placeholder: %w", err)
	}

	initPath := filepath.Join(p.rootfsDir, "init")
	if err := CopyFile(initBinPath, initPath); err != nil {
		return fmt.Errorf("copy init file: %w", err)
	}
	if err := os.Chmod(initPath, 0o755); err != nil {
		return fmt.Errorf("set init file permissions: %w", err)
	}

	moduleIndex, err := p.indexModules()
	if err != nil {
		return err
	}
	builtin, err := p.builtinModules()
	if err != nil {
		return err
	}

	modulesRoot := filepath.Join(p.rootfsDir, "lib", "modules", p.cfg.KernelRelease)
	if err := os.MkdirAll(modulesRoot, 0o755); err != nil {
		return err
	}
	orderFile, err := os.Create(filepath.Join(modulesRoot, "modules.order"))
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = orderFile.Close()
		}
	}()

	included := make(map[string]bool)
	fmt.Printf("Pruning kernel modules for release %q\n", p.cfg.KernelRelease)
	for _, modName := range p.cfg.RequiredModules {
		if err := p.includeModule(orderFile, modulesRoot, moduleIndex, builtin, included, modName); err != nil {
			return err
		}
	}
	if err := orderFile.Close(); err != nil {
		return fmt.Errorf("close modules.order: %w", err)
	}
	closed = true

	fmt.Printf("rootfs prepared: %s\n", p.rootfsDir)
	return nil
}

func ensureFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o600)
	if err != nil {
		return err
	}
	return f.Close()
}

func (p *ModulePruner) includeModule(orderFile *os.File, modulesRoot string, moduleIndex map[string]moduleCandidate, builtin, included map[string]bool, modName string) error {
	if candidate, ok := moduleIndex[modName]; ok {
		return p.copyModuleWithDependencies(orderFile, modulesRoot, moduleIndex, included, modName, candidate)
	}
	if builtin[modName] {
		fmt.Printf("module built in: %s\n", modName)
		return nil
	}
	return fmt.Errorf("required module %q is neither a module nor built into kernel release %q", modName, p.cfg.KernelRelease)
}

// builtinModules returns the names of the modules compiled into the kernel,
// from modules.builtin, which lists them by path such as
// kernel/drivers/block/virtio_blk.ko.
func (p *ModulePruner) builtinModules() (map[string]bool, error) {
	data, err := os.ReadFile(filepath.Join(p.extractedModulesDir, "modules.builtin"))
	if err != nil {
		return nil, fmt.Errorf("read built-in module list: %w", err)
	}
	builtin := make(map[string]bool)
	for _, entry := range strings.Fields(string(data)) {
		if name := moduleNameFromFile(filepath.Base(entry)); name != "" {
			builtin[name] = true
		}
	}
	return builtin, nil
}

func (p *ModulePruner) copyModuleWithDependencies(orderFile *os.File, modulesRoot string, moduleIndex map[string]moduleCandidate, included map[string]bool, modName string, candidate moduleCandidate) error {
	if included[modName] {
		return nil
	}
	depends, err := moduleDepends(candidate.sourcePath)
	if err != nil {
		return fmt.Errorf("read dependencies of module %s: %w", modName, err)
	}
	for _, depName := range depends {
		depCandidate, ok := moduleIndex[depName]
		if !ok {
			return fmt.Errorf("module %q depends on missing module %q", modName, depName)
		}
		if err := p.copyModuleWithDependencies(orderFile, modulesRoot, moduleIndex, included, depName, depCandidate); err != nil {
			return err
		}
	}
	return p.copyModule(orderFile, modulesRoot, included, modName, candidate)
}

func (p *ModulePruner) copyModule(orderFile *os.File, modulesRoot string, included map[string]bool, modName string, candidate moduleCandidate) error {
	dstKoPath := filepath.Join(modulesRoot, candidate.moduleRel)
	if err := decompressAndCopy(candidate.sourcePath, dstKoPath); err != nil {
		return fmt.Errorf("copy module %s: %w", modName, err)
	}
	if _, err := orderFile.WriteString(filepath.ToSlash(candidate.moduleRel) + "\n"); err != nil {
		return fmt.Errorf("write modules.order: %w", err)
	}
	included[modName] = true
	fmt.Printf("module included: %s\n", modName)
	return nil
}

func (p *ModulePruner) indexModules() (map[string]moduleCandidate, error) {
	moduleIndex := make(map[string]moduleCandidate)
	err := filepath.Walk(p.extractedModulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := moduleNameFromFile(info.Name())
		if name == "" {
			return nil
		}
		rel, err := filepath.Rel(p.extractedModulesDir, path)
		if err != nil {
			return err
		}
		moduleIndex[name] = moduleCandidate{
			sourcePath: path,
			moduleRel:  normalizeModuleRel(rel),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return moduleIndex, nil
}

func moduleNameFromFile(name string) string {
	switch {
	case strings.HasSuffix(name, ".ko"):
		return strings.TrimSuffix(name, ".ko")
	case strings.HasSuffix(name, ".ko.zst"):
		return strings.TrimSuffix(name, ".ko.zst")
	case strings.HasSuffix(name, ".ko.xz"):
		return strings.TrimSuffix(name, ".ko.xz")
	default:
		return ""
	}
}

func normalizeModuleRel(rel string) string {
	rel = strings.TrimSuffix(rel, ".zst")
	rel = strings.TrimSuffix(rel, ".xz")
	return rel
}

func moduleDepends(path string) ([]string, error) {
	data, err := moduleBytes(path)
	if err != nil {
		return nil, err
	}
	for _, line := range bytes.Split(data, []byte{0}) {
		dependsLine := string(line)
		if !strings.HasPrefix(dependsLine, "depends=") {
			continue
		}
		return parseDepends(strings.TrimPrefix(dependsLine, "depends=")), nil
	}
	return nil, nil
}

func moduleBytes(path string) ([]byte, error) {
	switch {
	case strings.HasSuffix(path, ".zst"):
		return exec.CommandContext(context.Background(), "zstd", "-dc", path).Output()
	case strings.HasSuffix(path, ".xz"):
		return exec.CommandContext(context.Background(), "xz", "-dc", path).Output()
	default:
		return os.ReadFile(path)
	}
}

func parseDepends(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	depends := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			depends = append(depends, part)
		}
	}
	return depends
}

func decompressAndCopy(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	switch {
	case strings.HasSuffix(src, ".zst"):
		cmd := exec.CommandContext(context.Background(), "zstd", "-d", "-f", src, "-o", dst)
		if out, err := cmd.CombinedOutput(); err != nil {
			_ = os.Remove(dst)
			return fmt.Errorf("zstd decompress failed: %w, output: %s", err, string(out))
		}
		return nil
	case strings.HasSuffix(src, ".xz"):
		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		cmd := exec.CommandContext(context.Background(), "xz", "-d", "-c", "-f", src)
		cmd.Stdout = out
		cmd.Stderr = os.Stderr
		cmdErr := cmd.Run()
		closeErr := out.Close()
		if cmdErr != nil {
			_ = os.Remove(dst)
			return fmt.Errorf("xz decompress failed: %w", cmdErr)
		}
		if closeErr != nil {
			_ = os.Remove(dst)
			return closeErr
		}
		return nil
	default:
		return CopyFile(src, dst)
	}
}
