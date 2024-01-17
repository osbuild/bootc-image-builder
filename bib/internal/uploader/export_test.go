package uploader

import (
	"io"
)

func MockOsStdout(new io.Writer) (restore func()) {
	saved := osStdout
	osStdout = new
	return func() {
		osStdout = saved
	}
}
