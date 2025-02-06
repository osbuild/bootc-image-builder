package progress

import (
	"io"
	"sync"
)

type syncedMultiWriter struct {
	mu sync.Mutex
	mw io.Writer
}

func newSyncedMultiWriter(wl ...io.Writer) io.Writer {
	return &syncedMultiWriter{mw: io.MultiWriter(wl...)}
}

func (sw *syncedMultiWriter) Write(p []byte) (n int, err error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	return sw.mw.Write(p)
}
