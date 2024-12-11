package progress

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/cheggaaa/pb/v3"
	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/osbuild"
)

var (
	osStderr         io.Writer = os.Stderr
	isattyIsTerminal           = isatty.IsTerminal
)

// ProgressBar is an interfacs for progress reporting when there is
// an arbitrary amount of sub-progress information (like osbuild)
type ProgressBar interface {
	// SetProgress sets the progress details at the given "level".
	// Levels should start with "0" and increase as the nesting
	// gets deeper.
	SetProgress(level int, msg string, done int, total int) error
	// The high-level message that is displayed in a spinner
	// (e.g. "Building image foo")
	SetPulseMsg(fmt string, args ...interface{})
	// A high level message with the last high level status
	// (e.g. "Started downloading")
	SetMessage(fmt string, args ...interface{})
	Start() error
	Stop() error
}

// New creates a new progressbar based on the requested type
func New(typ string) (ProgressBar, error) {
	switch typ {
	case "":
		// auto-select
		if f, ok := osStderr.(*os.File); ok {
			if isatty.IsTerminal(f.Fd()) {
				return NewTermProgressBar()
			}
		}
		return NewPlainProgressBar()
	case "plain":
		return NewPlainProgressBar()
	case "term":
		return NewTermProgressBar()
	case "debug":
		return NewDebugProgressBar()
	default:
		return nil, fmt.Errorf("unknown progress type: %q", typ)
	}
}

type termProgressBar struct {
	spinnerPb   *pb.ProgressBar
	msgPb       *pb.ProgressBar
	subLevelPbs []*pb.ProgressBar

	pool        *pb.Pool
	poolStarted bool
}

// NewProgressBar creates a new default pb3 based progressbar suitable for
// most terminals.
func NewTermProgressBar() (ProgressBar, error) {
	f, ok := osStderr.(*os.File)
	if !ok {
		return nil, fmt.Errorf("cannot use %T as a terminal", f)
	}
	if !isattyIsTerminal(f.Fd()) {
		return nil, fmt.Errorf("cannot use term progress without a terminal")
	}

	ppb := &termProgressBar{}
	ppb.spinnerPb = pb.New(0)
	ppb.spinnerPb.SetTemplate(`[{{ (cycle . "|" "/" "-" "\\") }}] {{ string . "spinnerMsg" }}`)
	ppb.msgPb = pb.New(0)
	ppb.msgPb.SetTemplate(`Message: {{ string . "msg" }}`)
	ppb.pool = pb.NewPool(ppb.spinnerPb, ppb.msgPb)
	ppb.pool.Output = osStderr
	return ppb, nil
}

func (ppb *termProgressBar) SetProgress(subLevel int, msg string, done int, total int) error {
	// auto-add as needed, requires sublevels to get added in order
	// i.e. adding 0 and then 2 will fail
	switch {
	case subLevel == len(ppb.subLevelPbs):
		apb := pb.New(0)
		ppb.subLevelPbs = append(ppb.subLevelPbs, apb)
		progressBarTmpl := `[{{ counters . }}] {{ string . "prefix" }} {{ bar .}} {{ percent . }}`
		apb.SetTemplateString(progressBarTmpl)
		ppb.pool.Add(apb)
	case subLevel > len(ppb.subLevelPbs):
		return fmt.Errorf("sublevel added out of order, have %v sublevels but want level %v", len(ppb.subLevelPbs), subLevel)
	}
	apb := ppb.subLevelPbs[subLevel]
	apb.SetTotal(int64(total) + 1)
	apb.SetCurrent(int64(done) + 1)
	if msg != "" {
		apb.Set("prefix", msg)
	}
	return nil
}

func shorten(msg string) string {
	msg = strings.Replace(msg, "\n", " ", -1)
	// XXX: make this smarter
	if len(msg) > 60 {
		return msg[:60] + "..."
	}
	return msg
}

func (ppb *termProgressBar) SetPulseMsg(msg string, args ...interface{}) {
	ppb.spinnerPb.Set("spinnerMsg", shorten(fmt.Sprintf(msg, args...)))
}

func (ppb *termProgressBar) SetMessage(msg string, args ...interface{}) {
	ppb.msgPb.Set("msg", shorten(fmt.Sprintf(msg, args...)))
}

func (ppb *termProgressBar) Start() error {
	if err := ppb.pool.Start(); err != nil {
		return fmt.Errorf("progress bar failed: %w", err)
	}
	ppb.poolStarted = true
	return nil
}

func (ppb *termProgressBar) Stop() (err error) {
	// pb.Stop() will deadlock if it was not started before
	if ppb.poolStarted {
		err = ppb.pool.Stop()
	}
	ppb.poolStarted = false
	return err
}

type plainProgressBar struct {
	w io.Writer
}

// NewPlainProgressBar starts a new "plain" progressbar that will just
// prints message but does not show any progress.
func NewPlainProgressBar() (ProgressBar, error) {
	np := &plainProgressBar{w: osStderr}
	return np, nil
}

func (np *plainProgressBar) SetPulseMsg(msg string, args ...interface{}) {
	fmt.Fprintf(np.w, msg, args...)
}

func (np *plainProgressBar) SetMessage(msg string, args ...interface{}) {
	fmt.Fprintf(np.w, msg, args...)
}

func (np *plainProgressBar) Start() (err error) {
	return nil
}

func (np *plainProgressBar) Stop() (err error) {
	return nil
}

func (np *plainProgressBar) SetProgress(subLevel int, msg string, done int, total int) error {
	return nil
}

type debugProgressBar struct {
	w io.Writer
}

// NewDebugProgressBar will create a progressbar aimed to debug the
// lower level osbuild/images message. It will never clear the screen
// so "glitches/weird" messages from the lower-layers can be inspected
// easier.
func NewDebugProgressBar() (ProgressBar, error) {
	np := &debugProgressBar{w: osStderr}
	return np, nil
}

func (np *debugProgressBar) SetPulseMsg(msg string, args ...interface{}) {
	fmt.Fprintf(np.w, "pulse: ")
	fmt.Fprintf(np.w, msg, args...)
	fmt.Fprintf(np.w, "\n")
}

func (np *debugProgressBar) SetMessage(msg string, args ...interface{}) {
	fmt.Fprintf(np.w, "msg: ")
	fmt.Fprintf(np.w, msg, args...)
	fmt.Fprintf(np.w, "\n")
}

func (np *debugProgressBar) Start() (err error) {
	fmt.Fprintf(np.w, "Start progressbar\n")
	return nil
}

func (np *debugProgressBar) Stop() (err error) {
	fmt.Fprintf(np.w, "Stop progressbar\n")
	return nil
}

func (np *debugProgressBar) SetProgress(subLevel int, msg string, done int, total int) error {
	fmt.Fprintf(np.w, "%s[%v / %v] %s", strings.Repeat("  ", subLevel), done, total, msg)
	fmt.Fprintf(np.w, "\n")
	return nil
}

// XXX: merge variant back into images/pkg/osbuild/osbuild-exec.go
func RunOSBuild(pb ProgressBar, manifest []byte, store, outputDirectory string, exports, extraEnv []string) error {
	switch pb.(type) {
	case *termProgressBar, *debugProgressBar:
		return runOSBuildNew(pb, manifest, store, outputDirectory, exports, extraEnv)
	default:
		return runOSBuildOld(pb, manifest, store, outputDirectory, exports, extraEnv)
	}
}

func runOSBuildOld(pb ProgressBar, manifest []byte, store, outputDirectory string, exports, extraEnv []string) error {
	_, err := osbuild.RunOSBuild(manifest, store, outputDirectory, exports, nil, extraEnv, false, os.Stderr)
	return err
}

func runOSBuildNew(pb ProgressBar, manifest []byte, store, outputDirectory string, exports, extraEnv []string) error {
	rp, wp, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("cannot create pipe for osbuild: %w", err)
	}
	defer rp.Close()
	defer wp.Close()

	cmd := exec.Command(
		"osbuild",
		"--store", store,
		"--output-directory", outputDirectory,
		"--monitor=JSONSeqMonitor",
		"--monitor-fd=3",
		"-",
	)
	for _, export := range exports {
		cmd.Args = append(cmd.Args, "--export", export)
	}

	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdin = bytes.NewBuffer(manifest)
	cmd.Stderr = os.Stderr
	// we could use "--json" here and would get the build-result
	// exported here
	cmd.Stdout = nil
	cmd.ExtraFiles = []*os.File{wp}

	osbuildStatus := osbuild.NewStatusScanner(rp)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting osbuild: %v", err)
	}
	wp.Close()

	var tracesMsgs []string
	for {
		st, err := osbuildStatus.Status()
		if err != nil {
			return fmt.Errorf("error reading osbuild status: %w", err)
		}
		if st == nil {
			break
		}
		i := 0
		for p := st.Progress; p != nil; p = p.SubProgress {
			// XXX: osbuild gives us bad progress messages
			if err := pb.SetProgress(i, p.Message, p.Done, p.Total); err != nil {
				logrus.Warnf("cannot set progress: %v", err)
			}
			i++
		}
		// keep the messages/traces for better error reporting
		if st.Message != "" {
			tracesMsgs = append(tracesMsgs, st.Message)
		}
		if st.Trace != "" {
			tracesMsgs = append(tracesMsgs, st.Trace)
		}
		// forward to user
		if st.Message != "" {
			pb.SetMessage(st.Message)
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("error running osbuild: %w\nLog:\n%s", err, strings.Join(tracesMsgs, "\n"))
	}

	return nil
}
