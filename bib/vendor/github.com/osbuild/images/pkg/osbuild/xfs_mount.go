package osbuild

func NewXfsMount(name, source, target string) *Mount {
	return &Mount{
		Type:   "org.osbuild.xfs",
		Name:   name,
		Source: source,
		Target: target,
	}
}
