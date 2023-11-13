package shell

type EnvironmentVariable struct {
	Key   string
	Value string
}

type InitFile struct {
	Filename  string
	Variables []EnvironmentVariable
}
