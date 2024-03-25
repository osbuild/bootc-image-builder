package setup

var (
	ValidateHasContainerStorageMountedFromReader = validateHasContainerStorageMountedFromReader
)

func MockInsideContainer(f func() (bool, error)) (restore func()) {
	saved := insideContainer
	insideContainer = f
	return func() {
		insideContainer = saved
	}
}
