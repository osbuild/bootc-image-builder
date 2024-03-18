package main

var (
	CanChownInPath                       = canChownInPath
	NewFilesystemCustomizationFromString = newFilesystemCustomizationFromString
)

func MockOsGetuid(new func() int) (restore func()) {
	saved := osGetuid
	osGetuid = new
	return func() {
		osGetuid = saved
	}
}
