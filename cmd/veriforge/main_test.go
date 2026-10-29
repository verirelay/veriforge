package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckRequired(t *testing.T) {
	valid := options{initBinPath: "init", verirelayRepository: "https://github.com/verirelay/verirelay", verirelayCommit: "0123456789abcdef0123456789abcdef01234567", kernelAppend: "console=ttyS0"}
	if err := valid.checkRequired(); err != nil {
		t.Fatalf("checkRequired returned error: %v", err)
	}
	for name, unset := range map[string]func(*options){
		"-init":                 func(o *options) { o.initBinPath = "" },
		"-verirelay-repository": func(o *options) { o.verirelayRepository = "" },
		"-verirelay-commit":     func(o *options) { o.verirelayCommit = "" },
		"-kernel-append":        func(o *options) { o.kernelAppend = "" },
	} {
		t.Run(name, func(t *testing.T) {
			o := valid
			unset(&o)
			if err := o.checkRequired(); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("checkRequired error = %v, want %s is required", err, name)
			}
		})
	}
}

func TestValidateOutputPathsRejectsInitPath(t *testing.T) {
	dir := t.TempDir()
	initPath := filepath.Join(dir, "init")
	if err := validateOutputPaths(initPath, filepath.Join(dir, "vmlinuz"), filepath.Join(dir, "initrd.img"), initPath); err == nil {
		t.Fatal("validateOutputPaths succeeded, want init path conflict")
	}
}

func TestValidateOutputPathsRejectsDuplicateOutputs(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "artifact")
	if err := validateOutputPaths(filepath.Join(dir, "init"), out, out, filepath.Join(dir, "build-report.json")); err == nil {
		t.Fatal("validateOutputPaths succeeded, want duplicate output conflict")
	}
}
