package progress

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/osbuild"
)

var (
	osStderr io.Writer = os.Stderr

	// This is only needed because pb.Pool require a real terminal.
	// It sets it into "raw-mode" but there is really no need for
	// this (see "func render()" below) so once this is fixed
	// upstream we should remove this.
	ESC         = "\x1b"
	ERASE_LINE  = ESC + "[2K"
	CURSOR_HIDE = ESC + "[?25l"
	CURSOR_SHOW = ESC + "[?25h"
)

func cursorUp(i int) string {
	return fmt.Sprintf("%s[%dA", ESC, i)
}

// ProgressBar is an interface for progress reporting when there is
// an arbitrary amount of sub-progress information (like osbuild)
type ProgressBar interface {
	// SetProgress sets the progress details at the given "level".
	// Levels should start with "0" and increase as the nesting
	// gets deeper.
	//
	// Note that reducing depth is currently not supported, once
	// a sub-progress is added it cannot be removed/hidden
	// (but if required it can be added, its a SMOP)
	SetProgress(level int, msg string, done int, total int) error

	// The high-level message that is displayed in a spinner
	// that contains the current top level step, for bib this
	// is really just "Manifest generation step" and
	// "Image generation step". We could map this to a three-level
	// progress as well but we spend 90% of the time in the
	// "Image generation step" so the UI looks a bit odd.
	SetPulseMsgf(fmt string, args ...interface{})

	// A high level message with the last operation status.
	// For us this usually comes from the stages and has information
	// like "Starting module org.osbuild.selinux"
	SetMessagef(fmt string, args ...interface{})

	// Start will start rendering the progress information
	Start()

	// Stop will stop rendering the progress information, the
	// screen is not cleared, the last few lines will be visible
	Stop()
}

// New creates a new progressbar based on the requested type
func New(typ string) (ProgressBar, error) {
	switch typ {
	// XXX: autoseelct based on PS1 value (i.e. use term in
	// interactive shells only?)
	case "", "plain":
		return NewPlainProgressBar()
	case "term":
		return NewTerminalProgressBar()
	case "debug":
		return NewDebugProgressBar()
	default:
		return nil, fmt.Errorf("unknown progress type: %q", typ)
	}
}

type terminalProgressBar struct {
	spinnerPb   *pb.ProgressBar
	msgPb       *pb.ProgressBar
	subLevelPbs []*pb.ProgressBar

	shutdownCh chan bool

	out io.Writer
}

// NewTerminalProgressBar creates a new default pb3 based progressbar suitable for
// most terminals.
func NewTerminalProgressBar() (ProgressBar, error) {
	b := &terminalProgressBar{
		out: osStderr,
	}
	b.spinnerPb = pb.New(0)
	b.spinnerPb.SetTemplate(`[{{ (cycle . "|" "/" "-" "\\") }}] {{ string . "spinnerMsg" }}`)
	b.msgPb = pb.New(0)
	b.msgPb.SetTemplate(`Message: {{ string . "msg" }}`)
	return b, nil
}

func (b *terminalProgressBar) SetProgress(subLevel int, msg string, done int, total int) error {
	// auto-add as needed, requires sublevels to get added in order
	// i.e. adding 0 and then 2 will fail
	switch {
	case subLevel == len(b.subLevelPbs):
		apb := pb.New(0)
		progressBarTmpl := `[{{ counters . }}] {{ string . "prefix" }} {{ bar .}} {{ percent . }}`
		apb.SetTemplateString(progressBarTmpl)
		if err := apb.Err(); err != nil {
			return fmt.Errorf("error setting the progressbarTemplat: %w", err)
		}
		// workaround bug when running tests in tmt
		if apb.Width() == 0 {
			// this is pb.defaultBarWidth
			apb.SetWidth(100)
		}
		b.subLevelPbs = append(b.subLevelPbs, apb)
	case subLevel > len(b.subLevelPbs):
		return fmt.Errorf("sublevel added out of order, have %v sublevels but want level %v", len(b.subLevelPbs), subLevel)
	}
	apb := b.subLevelPbs[subLevel]
	apb.SetTotal(int64(total) + 1)
	apb.SetCurrent(int64(done) + 1)
	apb.Set("prefix", msg)
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

func (b *terminalProgressBar) SetPulseMsgf(msg string, args ...interface{}) {
	b.spinnerPb.Set("spinnerMsg", shorten(fmt.Sprintf(msg, args...)))
}

func (b *terminalProgressBar) SetMessagef(msg string, args ...interface{}) {
	b.msgPb.Set("msg", shorten(fmt.Sprintf(msg, args...)))
}

func (b *terminalProgressBar) render() {
	var renderedLines int
	fmt.Fprintf(b.out, "%s%s\n", ERASE_LINE, b.spinnerPb.String())
	renderedLines++
	for _, prog := range b.subLevelPbs {
		fmt.Fprintf(b.out, "%s%s\n", ERASE_LINE, prog.String())
		renderedLines++
	}
	fmt.Fprintf(b.out, "%s%s\n", ERASE_LINE, b.msgPb.String())
	renderedLines++
	fmt.Fprint(b.out, cursorUp(renderedLines))
}

// Workaround for the pb.Pool requiring "raw-mode" - see here how to avoid
// it. Once fixes upstream we should remove this.
func (b *terminalProgressBar) renderLoop() {
	for {
		select {
		case <-b.shutdownCh:
			b.render()
			// finally move cursor down again
			fmt.Fprint(b.out, CURSOR_SHOW)
			fmt.Fprint(b.out, strings.Repeat("\n", 2+len(b.subLevelPbs)))
			// close last to avoid race with b.out
			close(b.shutdownCh)
			return
		case <-time.After(200 * time.Millisecond):
			// break to redraw the screen
		}
		b.render()
	}
}

func (b *terminalProgressBar) Start() {
	// render() already running
	if b.shutdownCh != nil {
		return
	}
	fmt.Fprintf(b.out, "%s", CURSOR_HIDE)
	b.shutdownCh = make(chan bool)
	go b.renderLoop()
}

func (b *terminalProgressBar) Err() error {
	var errs []error
	if err := b.spinnerPb.Err(); err != nil {
		errs = append(errs, fmt.Errorf("error on spinner progressbar: %w", err))
	}
	if err := b.msgPb.Err(); err != nil {
		errs = append(errs, fmt.Errorf("error on spinner progressbar: %w", err))
	}
	for _, pb := range b.subLevelPbs {
		if err := pb.Err(); err != nil {
			errs = append(errs, fmt.Errorf("error on spinner progressbar: %w", err))
		}
	}
	return errors.Join(errs...)
}

func (b *terminalProgressBar) Stop() {
	if b.shutdownCh == nil {
		return
	}
	// request shutdown
	b.shutdownCh <- true
	// wait for ack
	select {
	case <-b.shutdownCh:
	// shudown complete
	case <-time.After(1 * time.Second):
		// I cannot think of how this could happen, i.e. why
		// closing would not work but lets be conservative -
		// without a timeout we hang here forever
		logrus.Warnf("no progress channel shutdown after 1sec")
	}
	b.shutdownCh = nil
	// This should never happen but be paranoid, this should
	// never happen but ensure we did not accumulate error while
	// running
	if err := b.Err(); err != nil {
		fmt.Fprintf(b.out, "error from pb.ProgressBar: %v", err)
	}
}

type plainProgressBar struct {
	w io.Writer
}

// NewPlainProgressBar starts a new "plain" progressbar that will just
// prints message but does not show any progress.
func NewPlainProgressBar() (ProgressBar, error) {
	b := &plainProgressBar{w: osStderr}
	return b, nil
}

func (b *plainProgressBar) SetPulseMsgf(msg string, args ...interface{}) {
	fmt.Fprintf(b.w, msg, args...)
}

func (b *plainProgressBar) SetMessagef(msg string, args ...interface{}) {
	fmt.Fprintf(b.w, msg, args...)
}

func (b *plainProgressBar) Start() {
}

func (b *plainProgressBar) Stop() {
}

func (b *plainProgressBar) SetProgress(subLevel int, msg string, done int, total int) error {
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
	b := &debugProgressBar{w: osStderr}
	return b, nil
}

func (b *debugProgressBar) SetPulseMsgf(msg string, args ...interface{}) {
	fmt.Fprintf(b.w, "pulse: ")
	fmt.Fprintf(b.w, msg, args...)
	fmt.Fprintf(b.w, "\n")
}

func (b *debugProgressBar) SetMessagef(msg string, args ...interface{}) {
	fmt.Fprintf(b.w, "msg: ")
	fmt.Fprintf(b.w, msg, args...)
	fmt.Fprintf(b.w, "\n")
}

func (b *debugProgressBar) Start() {
	fmt.Fprintf(b.w, "Start progressbar\n")
}

func (b *debugProgressBar) Stop() {
	fmt.Fprintf(b.w, "Stop progressbar\n")
}

func (b *debugProgressBar) SetProgress(subLevel int, msg string, done int, total int) error {
	fmt.Fprintf(b.w, "%s[%v / %v] %s", strings.Repeat("  ", subLevel), done, total, msg)
	fmt.Fprintf(b.w, "\n")
	return nil
}

// XXX: merge variant back into images/pkg/osbuild/osbuild-exec.go
func RunOSBuild(pb ProgressBar, manifest []byte, store, outputDirectory string, exports, extraEnv []string) error {
	// To keep maximum compatibility keep the old behavior to run osbuild
	// directly and show all messages unless we have a "real" progress bar.
	//
	// This should ensure that e.g. "podman bootc" keeps working as it
	// is currently expecting the raw osbuild output. Once we double
	// checked with them we can remove the runOSBuildNoProgress() and
	// just run with the new runOSBuildWithProgress() helper.
	switch pb.(type) {
	case *terminalProgressBar, *debugProgressBar:
		return runOSBuildWithProgress(pb, manifest, store, outputDirectory, exports, extraEnv)
	default:
		return runOSBuildNoProgress(pb, manifest, store, outputDirectory, exports, extraEnv)
	}
}

func runOSBuildNoProgress(pb ProgressBar, manifest []byte, store, outputDirectory string, exports, extraEnv []string) error {
	_, err := osbuild.RunOSBuild(manifest, store, outputDirectory, exports, nil, extraEnv, false, os.Stderr)
	return err
}

func runOSBuildWithProgress(pb ProgressBar, manifest []byte, store, outputDirectory string, exports, extraEnv []string) error {
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
			pb.SetMessagef(st.Message)
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("error running osbuild: %w\nLog:\n%s", err, strings.Join(tracesMsgs, "\n"))
	}

	return nil
}
