package osbuild

func NewBtrfsMount(name, source, target string) *Mount {
	return &Mount{
		Type:   "org.osbuild.btrfs",
		Name:   name,
		Source: source,
		Target: target,
	}
}
