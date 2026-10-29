package config

// KernelPackage describes the trusted kernel package input for the initrd build.
type KernelPackage struct {
	KernelURL     string
	ModulesURL    string
	KernelHash    string
	ModulesHash   string
	KernelRelease string

	// RequiredModules must each be a module of the kernel release, which the
	// initrd then ships with its dependencies, or be built in.
	RequiredModules []string
}

var DefaultKernel = KernelPackage{
	KernelURL:     "https://kernel.ubuntu.com/mainline/v6.18.59/amd64/linux-image-unsigned-6.18.59-061859-generic_6.18.59-061859.202610261534_amd64.deb",
	KernelHash:    "66a0f592ce532c166d6b4f08b235c6807ba7d5b000f43d854c3ded98daf101ae",
	KernelRelease: "6.18.59-061859-generic",

	ModulesURL:  "https://kernel.ubuntu.com/mainline/v6.18.59/amd64/linux-modules-6.18.59-061859-generic_6.18.59-061859.202610261534_amd64.deb",
	ModulesHash: "0f05ec7828ac34cc349f2e157f10a558503c265a59fbaeda0cab0ab755478e89",

	RequiredModules: []string{
		"ccp",
		"tsm_report",
		"sev-guest",
		"virtio_blk",
		"virtio_net",
	},
}
