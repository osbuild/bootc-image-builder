package osbuildprogress

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cheggaaa/pb/v3"
)

type OsbuildJsonProgress struct {
	ID      string `json:"id"`
	Context struct {
		Origin   string `json:"origin"`
		Pipeline struct {
			Name  string `json:"name"`
			Stage struct {
				Name string `json:"name"`
				ID   string `json:"id"`
			} `json:"stage"`
			ID string `json:"id"`
		} `json:"pipeline"`
	} `json:"context"`
	Progress struct {
		Name  string `json:"name"`
		Total int64  `json:"total"`
		Done  int64  `json:"done"`
		// XXX: there are currently only two levels but it should be
		// deeper nested in theory
		SubProgress struct {
			Name  string `json:"name"`
			Total int64  `json:"total"`
			Done  int64  `json:"done"`
			// XXX: in theory this could be more nested but it's not

		} `json:"progress"`
	} `json:"progress"`

	Message string `json:"message"`
}

func scanJsonSeq(r io.Reader, ch chan OsbuildJsonProgress, errCh chan error) {
	var progress OsbuildJsonProgress

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		// XXX: use a proper jsonseq reader?
		line := scanner.Bytes()
		line = bytes.Trim(line, "\x1e")
		if err := json.Unmarshal(line, &progress); err != nil {
			// XXX: provide an invalid lines chan
			errCh <- err
			continue
		}
		ch <- progress
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		errCh <- err
	}
}

func AttachProgress(r io.Reader, w io.Writer) {
	var progress OsbuildJsonProgress

	ch := make(chan OsbuildJsonProgress)
	errCh := make(chan error)
	go scanJsonSeq(r, ch, errCh)

	lastMessage := "-"

	spinnerPb := pb.New(0)
	spinnerPb.SetTemplate(`Building [{{ (cycle . "|" "/" "-" "\\") }}]`)
	mainPb := pb.New(0)
	progressBarTmplFmt := `[{{ counters . }}] %s: {{ string . "prefix" }} {{ bar .}} {{ percent . }}`
	mainPb.SetTemplateString(fmt.Sprintf(progressBarTmplFmt, "step"))
	subPb := pb.New(0)
	subPb.SetTemplateString(fmt.Sprintf(progressBarTmplFmt, "module"))
	msgPb := pb.New(0)
	msgPb.SetTemplate(`last msg: {{ string . "msg" }}`)

	pool, err := pb.StartPool(spinnerPb, mainPb, subPb, msgPb)
	if err != nil {
		fmt.Fprintf(os.Stderr, "progress failed: %v\n", err)
		return
	}

	contextMap := map[string]string{}

	for {
		select {
		case err := <-errCh:
			fmt.Fprintf(os.Stderr, "error: %v", err)
			break
		case progress = <-ch:
			id := progress.Context.Pipeline.ID
			pipelineName := contextMap[id]
			if pipelineName == "" {
				pipelineName = progress.Context.Pipeline.Name
				contextMap[id] = pipelineName
			}
			// XXX: use differentmap?
			id = "stage-" + progress.Context.Pipeline.Stage.ID
			stageName := contextMap[id]
			if stageName == "" {
				stageName = progress.Context.Pipeline.Stage.Name
				contextMap[id] = stageName
			}

			if progress.Progress.Total > 0 {
				mainPb.SetTotal(progress.Progress.Total + 1)
				mainPb.SetCurrent(progress.Progress.Done + 1)
				mainPb.Set("prefix", pipelineName)
			}
			// XXX: use context instead of name here too
			if progress.Progress.SubProgress.Total > 0 {
				subPb.SetTotal(progress.Progress.SubProgress.Total + 1)
				subPb.SetCurrent(progress.Progress.SubProgress.Done + 1)
				subPb.Set("prefix", strings.TrimPrefix(stageName, "org.osbuild."))
			}

			// todo: make message more structured in osbuild?
			// message from the stages themselfs are very noisy
			// best not to show to the user (only for failures)
			if progress.Context.Origin == "osbuild.monitor" {
				lastMessage = progress.Message
			}
			/*
				// todo: fix in osbuild?
				lastMessage = strings.TrimSpace(strings.SplitN(progress.Message, "\n", 2)[0])
				l := strings.SplitN(lastMessage, ":", 2)
				if len(l) > 1 {
					lastMessage = strings.TrimSpace(l[1])
				}
			*/
			msgPb.Set("msg", lastMessage)

		case <-time.After(200 * time.Millisecond):
			// nothing
		}
	}
	pool.Stop()
}

// XXX: merge back into images/pkg/osbuild/osbuild-exec.go(?)
func RunOSBuild(manifest []byte, store, outputDirectory string, exports, extraEnv []string) error {
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
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdin = bytes.NewBuffer(manifest)
	cmd.Stderr = os.Stderr
	// we could use "--json" here and would get the build-result
	// exported here
	cmd.Stdout = nil
	cmd.ExtraFiles = []*os.File{wp}
	go AttachProgress(rp, os.Stdout)

	for _, export := range exports {
		cmd.Args = append(cmd.Args, "--export", export)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting osbuild: %v", err)
	}
	wp.Close()

	// XXX: add WaitGroup
	return cmd.Wait()
}
