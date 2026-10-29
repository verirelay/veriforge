package main

import (
	"context"
	"debug/elf"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/verirelay/protocol/guest"
	"github.com/verirelay/veriforge/internal/config"
	"github.com/verirelay/veriforge/internal/forge"
	"github.com/verirelay/veriforge/internal/manifest"
	"github.com/verirelay/veriforge/internal/report"
)

func main() {
	os.Exit(run())
}

type options struct {
	initBinPath, verirelayRepository, verirelayCommit string
	vcpuType, kernelAppend                            string
	outInitrd, outVmlinuz, outReport, cacheDir        string
}

func parseFlags() options {
	initBinPath := flag.String("init", "", "required: path to the Linux amd64 static ELF that runs as micro-system PID 1")
	verirelayRepository := flag.String("verirelay-repository", "", "required: canonical HTTPS repository URL of the VeriRelay source")
	verirelayCommit := flag.String("verirelay-commit", "", "required: lowercase 40-char commit SHA for the VeriRelay binary")
	vcpuType := flag.String("vcpu-type", guest.VCPUType, "QEMU CPU profile used for the SEV-SNP launch measurement")
	kernelAppend := flag.String("kernel-append", "", "required: raw QEMU direct-boot kernel append string; OVMF auto-appended parameters are not included")
	outInitrd := flag.String("out-initrd", "initrd.img", "initrd output path")
	outVmlinuz := flag.String("out-vmlinuz", "vmlinuz", "vmlinuz output path")
	outReport := flag.String("out-report", "", "build-report.json output path; defaults to the initrd directory")
	cacheDir := flag.String("cache", "./.veriforge_cache", "cache directory for kernel and modules .deb files")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "VeriForge: deterministic Linux micro-system image builder\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  veriforge -init <linux-amd64-static-init> -verirelay-repository <url> -verirelay-commit <sha> -kernel-append <cmdline> [flags]\n\nFlags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *outReport == "" {
		*outReport = filepath.Join(filepath.Dir(*outInitrd), "build-report.json")
	}

	return options{
		initBinPath:         *initBinPath,
		verirelayRepository: *verirelayRepository,
		verirelayCommit:     *verirelayCommit,
		vcpuType:            *vcpuType,
		kernelAppend:        *kernelAppend,
		outInitrd:           *outInitrd,
		outVmlinuz:          *outVmlinuz,
		outReport:           *outReport,
		cacheDir:            *cacheDir,
	}
}

func (o options) checkRequired() error {
	for _, required := range []struct{ flag, value string }{
		{"-init", o.initBinPath},
		{"-verirelay-repository", o.verirelayRepository},
		{"-verirelay-commit", o.verirelayCommit},
		{"-kernel-append", o.kernelAppend},
	} {
		if required.value == "" {
			return fmt.Errorf("%s is required", required.flag)
		}
	}
	return nil
}

type buildInputs struct {
	manifest          guest.Manifest
	veriforgeIdentity manifest.GitIdentity
	manifestJSON      []byte
}

func prepareInputs(o options) (buildInputs, error) {
	if err := validateReadableRegularFile(o.initBinPath); err != nil {
		return buildInputs{}, fmt.Errorf("invalid init binary path: %w", err)
	}
	if err := validateInitBinary(o.initBinPath); err != nil {
		return buildInputs{}, fmt.Errorf("invalid init binary: %w", err)
	}
	if err := validateOutputPaths(o.initBinPath, o.outVmlinuz, o.outInitrd, o.outReport); err != nil {
		return buildInputs{}, fmt.Errorf("invalid output path: %w", err)
	}

	m, veriforgeIdentity, manifestJSON, err := manifest.New(manifest.Inputs{
		VeriRelayRepository: o.verirelayRepository,
		VeriRelayCommit:     o.verirelayCommit,
		VCPUType:            o.vcpuType,
		KernelAppend:        o.kernelAppend,
		RepoDir:             ".",
	})
	if err != nil {
		return buildInputs{}, fmt.Errorf("build manifest: %w", err)
	}

	if err := checkRequiredCommands("dpkg-deb", "zstd", "xz"); err != nil {
		return buildInputs{}, fmt.Errorf("dependency check failed: %w", err)
	}

	return buildInputs{
		manifest:          m,
		veriforgeIdentity: veriforgeIdentity,
		manifestJSON:      manifestJSON,
	}, nil
}

func printBuildHeader(in buildInputs) {
	fmt.Println("Starting VeriForge build")
	fmt.Printf("manifest path in initrd: /%s\n", manifest.Path)
	if in.veriforgeIdentity.Dirty {
		fmt.Fprintln(os.Stderr, "warning: VeriForge git worktree is dirty; build report will record dirty=true")
	}
	fmt.Println("manifest:")
	fmt.Print(string(in.manifestJSON))
}

func buildArtifacts(o options, in buildInputs) error {
	tempDir, err := os.MkdirTemp("", "veriforge_rootfs_*")
	if err != nil {
		return fmt.Errorf("create temporary work directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	assetsDir := filepath.Join(tempDir, "assets")
	rootfsDir := filepath.Join(tempDir, "rootfs")

	extractor := forge.NewExtractor(config.DefaultKernel, o.cacheDir)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	extractedVmlinuz, fullModulesDir, err := extractor.PrepareArtifacts(ctx, assetsDir)
	stop()
	if err != nil {
		return fmt.Errorf("prepare kernel artifacts: %w", err)
	}

	if err := forge.CopyFile(extractedVmlinuz, o.outVmlinuz); err != nil {
		return fmt.Errorf("write vmlinuz output: %w", err)
	}
	fmt.Printf("vmlinuz written: %s\n", o.outVmlinuz)

	pruner := forge.NewModulePruner(fullModulesDir, rootfsDir, config.DefaultKernel)
	if err := pruner.PruneAndBuildRootfs(o.initBinPath); err != nil {
		return fmt.Errorf("build rootfs: %w", err)
	}
	if err := forge.InstallCACertificates(rootfsDir); err != nil {
		return fmt.Errorf("install CA certificates: %w", err)
	}
	if err := manifest.WriteFile(rootfsDir, in.manifestJSON); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	archiver := forge.NewDeterministicArchiver(rootfsDir)
	if err := archiver.BuildInitrd(o.outInitrd); err != nil {
		return fmt.Errorf("build initrd: %w", err)
	}
	initAbs, err := filepath.Abs(o.initBinPath)
	if err != nil {
		return fmt.Errorf("resolve init binary path: %w", err)
	}
	if err := manifest.VerifyInInitrd(o.outInitrd, in.manifest, o.initBinPath, initAbs); err != nil {
		return fmt.Errorf("verify manifest: %w", err)
	}

	return nil
}

func writeBuildReport(o options, in buildInputs) error {
	_, reportJSON, err := report.New(report.Inputs{
		Manifest:      in.manifest,
		ManifestBytes: in.manifestJSON,
		VeriForge:     in.veriforgeIdentity,
		Kernel:        config.DefaultKernel,
		BinaryPath:    o.initBinPath,
		KernelPath:    o.outVmlinuz,
		InitrdPath:    o.outInitrd,
	})
	if err != nil {
		return fmt.Errorf("build report: %w", err)
	}
	if err := report.WriteFile(o.outReport, reportJSON); err != nil {
		return fmt.Errorf("write build report: %w", err)
	}

	return nil
}

func printSummary(o options) {
	fmt.Println("Build completed")
	fmt.Printf("kernel: %s\n", o.outVmlinuz)
	fmt.Printf("initrd: %s\n", o.outInitrd)
	fmt.Printf("build report: %s\n", o.outReport)
	if d, err := report.DigestFile(o.outInitrd); err == nil {
		fmt.Printf("initrd sha256: %s\n", d.SHA256)
	}
}

func run() int {
	o := parseFlags()
	if err := o.checkRequired(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		flag.Usage()
		return 1
	}

	in, err := prepareInputs(o)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	printBuildHeader(in)
	if err := buildArtifacts(o, in); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if err := writeBuildReport(o, in); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	printSummary(o)
	return 0
}

func validateOutputPaths(initPath, outVmlinuz, outInitrd, outReport string) error {
	initAbs, err := filepath.Abs(initPath)
	if err != nil {
		return err
	}
	outputs := map[string]string{}
	for _, outputPath := range []string{outVmlinuz, outInitrd, outReport} {
		outputAbs, err := filepath.Abs(outputPath)
		if err != nil {
			return err
		}
		if outputAbs == initAbs {
			return fmt.Errorf("output path %q must not be the same as -init", outputPath)
		}
		if previous, ok := outputs[outputAbs]; ok {
			return fmt.Errorf("output path %q must not be the same as %q", outputPath, previous)
		}
		outputs[outputAbs] = outputPath
	}
	return nil
}

func validateReadableRegularFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	return f.Close()
}

func validateInitBinary(path string) error {
	f, err := elf.Open(path)
	if err != nil {
		return fmt.Errorf("%q is not an ELF binary: %w", path, err)
	}
	defer func() {
		_ = f.Close()
	}()

	if f.Machine != elf.EM_X86_64 {
		return fmt.Errorf("%q is %s, want x86-64 ELF", path, f.Machine)
	}
	for _, prog := range f.Progs {
		if prog.Type == elf.PT_INTERP {
			return fmt.Errorf("%q is dynamically linked; provide a static Linux amd64 binary", path)
		}
	}
	return nil
}

func checkRequiredCommands(commands ...string) error {
	for _, command := range commands {
		if _, err := exec.LookPath(command); err != nil {
			return fmt.Errorf("missing command %q", command)
		}
	}
	return nil
}
