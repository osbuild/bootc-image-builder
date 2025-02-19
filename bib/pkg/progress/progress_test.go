package progress_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/bootc-image-builder/bib/pkg/progress"
)

func TestProgressNew(t *testing.T) {
	for _, tc := range []struct {
		typ         string
		expected    interface{}
		expectedErr string
	}{
		{"term", &progress.TerminalProgressBar{}, ""},
		{"debug", &progress.DebugProgressBar{}, ""},
		{"verbose", &progress.VerboseProgressBar{}, ""},
		// unknown progress type
		{"bad", nil, `unknown progress type: "bad"`},
	} {
		pb, err := progress.New(tc.typ)
		if tc.expectedErr == "" {
			assert.NoError(t, err)
			assert.Equal(t, reflect.TypeOf(pb), reflect.TypeOf(tc.expected), fmt.Sprintf("[%v] %T not the expected %T", tc.typ, pb, tc.expected))
		} else {
			assert.EqualError(t, err, tc.expectedErr)
		}
	}
}

func TestVerboseProgress(t *testing.T) {
	var buf bytes.Buffer
	restore := progress.MockOsStderr(&buf)
	defer restore()

	// verbose progress never generates progress output
	pbar, err := progress.NewVerboseProgressBar()
	assert.NoError(t, err)
	err = pbar.SetProgress(0, "set-progress", 1, 100)
	assert.NoError(t, err)
	assert.Equal(t, "", buf.String())

	// but it shows the messages
	pbar.SetPulseMsgf("pulse")
	assert.Equal(t, "pulse\n", buf.String())
	buf.Reset()

	pbar.SetMessagef("message")
	assert.Equal(t, "message\n", buf.String())
	buf.Reset()

	pbar.Start()
	assert.Equal(t, "", buf.String())
	pbar.Stop()
	assert.Equal(t, "", buf.String())
}

func TestDebugProgress(t *testing.T) {
	var buf bytes.Buffer
	restore := progress.MockOsStderr(&buf)
	defer restore()

	pbar, err := progress.NewDebugProgressBar()
	assert.NoError(t, err)
	err = pbar.SetProgress(0, "set-progress-msg", 1, 100)
	assert.NoError(t, err)
	assert.Equal(t, "[1 / 100] set-progress-msg\n", buf.String())
	buf.Reset()

	pbar.SetPulseMsgf("pulse-msg")
	assert.Equal(t, "pulse: pulse-msg\n", buf.String())
	buf.Reset()

	pbar.SetMessagef("some-message")
	assert.Equal(t, "msg: some-message\n", buf.String())
	buf.Reset()

	pbar.Start()
	assert.Equal(t, "Start progressbar\n", buf.String())
	buf.Reset()

	pbar.Stop()
	assert.Equal(t, "Stop progressbar\n", buf.String())
	buf.Reset()
}

func TestTermProgress(t *testing.T) {
	var buf bytes.Buffer
	restore := progress.MockOsStderr(&buf)
	defer restore()

	pbar, err := progress.NewTerminalProgressBar()
	assert.NoError(t, err)

	pbar.Start()
	pbar.SetPulseMsgf("pulse-msg")
	pbar.SetMessagef("some-message")
	err = pbar.SetProgress(0, "set-progress-msg", 0, 5)
	assert.NoError(t, err)
	pbar.Stop()
	assert.NoError(t, pbar.(*progress.TerminalProgressBar).Err())

	assert.Contains(t, buf.String(), "[1 / 6] set-progress-msg")
	assert.Contains(t, buf.String(), "[|] pulse-msg\n")
	assert.Contains(t, buf.String(), "Message: some-message\n")
	// check shutdown
	assert.Contains(t, buf.String(), progress.CURSOR_SHOW)
}

func TestProgressNewAutoselect(t *testing.T) {
	for _, tc := range []struct {
		onTerm   bool
		expected interface{}
	}{
		{false, &progress.VerboseProgressBar{}},
		{true, &progress.TerminalProgressBar{}},
	} {
		restore := progress.MockIsattyIsTerminal(func(uintptr) bool {
			return tc.onTerm
		})
		defer restore()

		pb, err := progress.New("auto")
		assert.NoError(t, err)
		assert.Equal(t, reflect.TypeOf(pb), reflect.TypeOf(tc.expected), fmt.Sprintf("[%v] %T not the expected %T", tc.onTerm, pb, tc.expected))
	}
}

func makeFakeOsbuild(t *testing.T, content string) string {
	p := filepath.Join(t.TempDir(), "fake-osbuild")
	err := os.WriteFile(p, []byte("#!/bin/sh\n"+content), 0755)
	assert.NoError(t, err)
	return p
}

func TestRunOSBuildWithProgressErrorReporting(t *testing.T) {
	restore := progress.MockOsStderr(io.Discard)
	defer restore()

	restore = progress.MockOsbuildCmd(makeFakeOsbuild(t, `
>&3 echo '{"message": "osbuild-stage-message"}'

echo osbuild-stdout-output
>&2 echo osbuild-stderr-output
exit 112
`))
	defer restore()

	pbar, err := progress.New("debug")
	assert.NoError(t, err)
	err = progress.RunOSBuild(pbar, []byte(`{"fake":"manifest"}`), nil, nil)
	assert.EqualError(t, err, `error running osbuild: exit status 112
BuildLog:
osbuild-stage-message
Output:
osbuild-stdout-output
osbuild-stderr-output
`)
}

func TestRunOSBuildWithProgressIncorrectJSON(t *testing.T) {
	signalDeliveredMarkerPath := filepath.Join(t.TempDir(), "sigint-delivered")

	restore := progress.MockOsbuildCmd(makeFakeOsbuild(t, fmt.Sprintf(`
trap 'touch "%s";exit 2' INT

>&3 echo invalid-json

# we cannot sleep infinity here or the shell script trap is never run
while true; do
    sleep 0.1
done
`, signalDeliveredMarkerPath)))
	defer restore()

	pbar, err := progress.New("debug")
	assert.NoError(t, err)
	err = progress.RunOSBuild(pbar, []byte(`{"fake":"manifest"}`), nil, nil)
	assert.EqualError(t, err, `error parsing osbuild status, please report a bug and try with "--progress=verbose": cannot scan line "invalid-json": invalid character 'i' looking for beginning of value`)

	// ensure the SIGINT got delivered
	var pathExists = func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if pathExists(signalDeliveredMarkerPath) {
			break
		}
	}
	assert.True(t, pathExists(signalDeliveredMarkerPath))
}

func TestRunOSBuildWithBuildlogTerm(t *testing.T) {
	restore := progress.MockOsbuildCmd(makeFakeOsbuild(t, `
echo osbuild-stdout-output
>&2 echo osbuild-stderr-output

# without the sleep this is racy as two different go routines poll
# this does not matter (much) in practise because osbuild output and
# stage output are using the syncedMultiWriter so output is not garbled
sleep 0.1
>&3 echo '{"message": "osbuild-stage-message"}'
`))
	defer restore()

	var fakeStdout, fakeStderr bytes.Buffer
	restore = progress.MockOsStdout(&fakeStdout)
	defer restore()
	restore = progress.MockOsStderr(&fakeStderr)
	defer restore()

	pbar, err := progress.New("term")
	assert.NoError(t, err)

	var buildLog bytes.Buffer
	opts := &progress.OSBuildOptions{
		BuildLog: &buildLog,
	}
	err = progress.RunOSBuild(pbar, []byte(`{"fake":"manifest"}`), nil, opts)
	assert.NoError(t, err)
	expectedOutput := `osbuild-stdout-output
osbuild-stderr-output
osbuild-stage-message
`
	assert.Equal(t, expectedOutput, buildLog.String())
}

func TestRunOSBuildWithBuildlogVerbose(t *testing.T) {
	restore := progress.MockOsbuildCmd(makeFakeOsbuild(t, `
echo osbuild-stdout-output
>&2 echo osbuild-stderr-output
`))
	defer restore()

	var fakeStdout, fakeStderr bytes.Buffer
	restore = progress.MockOsStdout(&fakeStdout)
	defer restore()
	restore = progress.MockOsStderr(&fakeStderr)
	defer restore()

	pbar, err := progress.New("verbose")
	assert.NoError(t, err)

	var buildLog bytes.Buffer
	opts := &progress.OSBuildOptions{
		BuildLog: &buildLog,
	}
	err = progress.RunOSBuild(pbar, []byte(`{"fake":"manifest"}`), nil, opts)
	assert.NoError(t, err)
	expectedOutput := `osbuild-stdout-output
osbuild-stderr-output
`
	assert.Equal(t, expectedOutput, buildLog.String())
}
