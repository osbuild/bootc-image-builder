package progress_test

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/bootc-image-builder/bib/pkg/progress"
)

func TestSyncWriter(t *testing.T) {
	var mu sync.Mutex
	var buf bytes.Buffer
	var wg sync.WaitGroup

	for id := 0; id < 100; id++ {
		wg.Add(1)
		w := progress.NewSyncedWriter(&mu, &buf)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				fmt.Fprintln(w, strings.Repeat(fmt.Sprintf("%v", id%10), 60))
				time.Sleep(10 * time.Nanosecond)
			}
		}(id)
	}
	wg.Wait()

	scanner := bufio.NewScanner(&buf)
	for {
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		assert.True(t, len(line) == 60, fmt.Sprintf("len %v: line: %v", len(line), line))
	}
	assert.NoError(t, scanner.Err())
}
