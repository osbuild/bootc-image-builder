package main

var (
	CanChownInPath                = canChownInPath
	ApplyFilesystemCustomizations = applyFilesystemCustomizations
	GetDistroAndRunner            = getDistroAndRunner
)

func MockOsGetuid(new func() int) (restore func()) {
	saved := osGetuid
	osGetuid = new
	return func() {
		osGetuid = saved
	}
}
