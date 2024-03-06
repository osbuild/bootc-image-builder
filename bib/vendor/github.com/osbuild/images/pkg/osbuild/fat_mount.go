package osbuild

func NewFATMount(name, source, target string) *Mount {
	return &Mount{
		Type:   "org.osbuild.fat",
		Name:   name,
		Source: source,
		Target: target,
	}
}
