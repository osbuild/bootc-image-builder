package main

var (
	CanChownInPath                = canChownInPath
	CheckFilesystemCustomizations = checkFilesystemCustomizations
	GetDistroAndRunner            = getDistroAndRunner
	CheckMountpoints              = checkMountpoints
	PartitionTables               = partitionTables
	UpdateFilesystemSizes         = updateFilesystemSizes
	GenPartitionTable             = genPartitionTable
	CreateRand                    = createRand
	BuildCobraCmdline             = buildCobraCmdline
	CalcRequiredDirectorySizes    = calcRequiredDirectorySizes
)

func MockOsGetuid(new func() int) (restore func()) {
	saved := osGetuid
	osGetuid = new
	return func() {
		osGetuid = saved
	}
}
