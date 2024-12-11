package progress

import (
	"io"
)

type (
	TermProgressBar  = termProgressBar
	DebugProgressBar = debugProgressBar
	PlainProgressBar = plainProgressBar
)

func MockOsStderr(w io.Writer) (restore func()) {
	saved := osStderr
	osStderr = w
	return func() {
		osStderr = saved
	}
}

func MockIsattyIsTerminal(b bool) (restore func()) {
	saved := isattyIsTerminal
	isattyIsTerminal = func(uintptr) bool {
		return b
	}
	return func() {
		isattyIsTerminal = saved
	}
}
