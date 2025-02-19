package progress

import (
	"io"
)

type (
	TerminalProgressBar = terminalProgressBar
	DebugProgressBar    = debugProgressBar
	VerboseProgressBar  = verboseProgressBar
)

var (
	NewSyncedWriter = newSyncedWriter
)

func MockOsStdout(w io.Writer) (restore func()) {
	saved := osStdout
	osStdout = func() io.Writer { return w }
	return func() {
		osStdout = saved
	}
}

func MockOsStderr(w io.Writer) (restore func()) {
	saved := osStderr
	osStderr = func() io.Writer { return w }
	return func() {
		osStderr = saved
	}
}

func MockIsattyIsTerminal(fn func(uintptr) bool) (restore func()) {
	saved := isattyIsTerminal
	isattyIsTerminal = fn
	return func() {
		isattyIsTerminal = saved
	}
}

func MockOsbuildCmd(s string) (restore func()) {
	saved := osbuildCmd
	osbuildCmd = s
	return func() {
		osbuildCmd = saved
	}
}
