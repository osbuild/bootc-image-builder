package osbuildprogress

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
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
		Total int    `json:"total"`
		Done  int    `json:"done"`
		// XXX: there are currently only two levels but it should be
		// deeper nested in theory
		SubProgress struct {
			Name  string `json:"name"`
			Total int    `json:"total"`
			Done  int    `json:"done"`
			// XXX: in theory this could be more nested but it's not

		} `json:"progress"`
	} `json:"progress"`

	Message string `json:"message"`
}

func scanJsonSeq(r io.Reader, ch chan OsbuildJsonProgress) {
	var progress OsbuildJsonProgress

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		// XXX: use a proper jsonseq reader
		line := scanner.Bytes()
		//println(string(line))
		line = bytes.Trim(line, "\x1e")
		if err := json.Unmarshal(line, &progress); err != nil {
			fmt.Fprintf(os.Stderr, "json decode err for %s: %v\n", line, err)
			continue
		}
		ch <- progress
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		// log error here
	}
}

func AttachProgress(r io.Reader, w io.Writer) {
	var progress OsbuildJsonProgress
	spinner := []string{"|", "/", "-", "\\"}
	i := 0

	ch := make(chan OsbuildJsonProgress)
	go scanJsonSeq(r, ch)

	mainProgress := "unknown"
	subProgress := ""
	message := "-"

	contextMap := map[string]string{}

	fmt.Fprintf(w, "\n")
	for {
		select {
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
				mainProgress = fmt.Sprintf("component: %v [%v/%v]", pipelineName, progress.Progress.Done+1, progress.Progress.Total+1)
			}
			// XXX: use context instead of name here too
			if progress.Progress.SubProgress.Total > 0 {
				subProgress = fmt.Sprintf("%v [%v/%v]", stageName, progress.Progress.SubProgress.Done+1, progress.Progress.SubProgress.Total+1)
			}

			// todo: make message more structured in osbuild?
			// message from the stages themselfs are very noisy
			// best not to show to the user (only for failures)
			if progress.Context.Origin == "osbuild.monitor" {
				message = progress.Message
			}
			//message = strings.TrimSpace(strings.SplitN(progress.Message, "\n", 2)[0])
			// todo: fix in osbuild?
			/*
				l := strings.SplitN(message, ":", 2)
				if len(l) > 1 {
					message = strings.TrimSpace(l[1])
				}
			*/
			if len(message) > 60 {
				message = message[:60] + "..."
			}
		case <-time.After(200 * time.Millisecond):
			// nothing
		}

		// XXX: use real progressbar *or* use helper to get terminal
		// size for proper length checks etc
		//
		// poor man progress, we need multiple progress bars and
		// a message that keeps getting updated (or maybe not the
		// message)
		fmt.Fprintf(w, "\x1b[2K[%s] %s\n", spinner[i], mainProgress)
		if subProgress != "" {
			fmt.Fprintf(w, "\x1b[2Ksub-progress: %s\n", subProgress)
		}
		fmt.Fprintf(w, "\x1b[2Kmessage: %s\n", message)
		if subProgress != "" {
			fmt.Fprintf(w, "\x1b[%dA", 3)
		} else {
			fmt.Fprintf(w, "\x1b[%dA", 2)
		}
		// spin
		i = (i + 1) % len(spinner)
	}
}

// XXX: merge back into images/pkg/osbuild/osbuild-exec.go
func RunOSBuild(manifest []byte, store, outputDirectory string, exports, extraEnv []string) error {
	cmd := exec.Command(
		"osbuild",
		"--store", store,
		"--output-directory", outputDirectory,
		"--monitor=JSONSeqMonitor",
		"-",
	)
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdin = bytes.NewBuffer(manifest)
	cmd.Stderr = os.Stderr

	for _, export := range exports {
		cmd.Args = append(cmd.Args, "--export", export)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	go AttachProgress(stdout, os.Stdout)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting osbuild: %v", err)
	}
	return cmd.Wait()
}
