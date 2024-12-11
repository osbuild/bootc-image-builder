package progress

import (
	"io"
)

type (
	TerminalProgressBar = terminalProgressBar
	DebugProgressBar    = debugProgressBar
	PlainProgressBar    = plainProgressBar
)

func MockOsStderr(w io.Writer) (restore func()) {
	saved := osStderr
	osStderr = w
	return func() {
		osStderr = saved
	}
}
