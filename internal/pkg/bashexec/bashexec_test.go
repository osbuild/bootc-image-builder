package bashexec

import "testing"

func TestBashExec(t *testing.T) {
	err := Run("true", "true")
	if err != nil {
		panic(err)
	}
	err = Run("task that should fail", "false")
	if err == nil {
		panic("expected err")
	}
}
