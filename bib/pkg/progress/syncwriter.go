package progress

import (
	"io"
	"sync"
)

type syncedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func newSyncedWriter(mu *sync.Mutex, w io.Writer) io.Writer {
	return &syncedWriter{mu: mu, w: w}
}

func (sw *syncedWriter) Write(p []byte) (n int, err error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	return sw.w.Write(p)
}
