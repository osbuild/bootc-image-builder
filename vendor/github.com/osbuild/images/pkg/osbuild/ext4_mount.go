package osbuild

func NewExt4Mount(name, source, target string) *Mount {
	return &Mount{
		Type:   "org.osbuild.ext4",
		Name:   name,
		Source: source,
		Target: target,
	}
}
