package runner

type Runner interface {
	String() string
	GetBuildPackages() []string
}
