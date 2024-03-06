package container

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type resolveResult struct {
	spec Spec
	err  error
}

type Resolver struct {
	jobs  int
	queue chan resolveResult

	ctx context.Context

	Arch         string
	AuthFilePath string
}

type SourceSpec struct {
	Source              string
	Name                string
	Digest              *string
	TLSVerify           *bool
	ContainersTransport *string
	StoragePath         *string
}

func NewResolver(arch string) *Resolver {
	return &Resolver{
		ctx:   context.Background(),
		queue: make(chan resolveResult, 2),
		Arch:  arch,
	}
}

func (r *Resolver) Add(spec SourceSpec) {
	client, err := NewClient(spec.Source)
	r.jobs += 1

	if err != nil {
		r.queue <- resolveResult{err: err}
		return
	}

	client.SetTLSVerify(spec.TLSVerify)
	client.SetArchitectureChoice(r.Arch)
	if r.AuthFilePath != "" {
		client.SetAuthFilePath(r.AuthFilePath)
	}

	go func() {
		spec, err := client.Resolve(r.ctx, spec.Name, &spec)
		if err != nil {
			err = fmt.Errorf("'%s': %w", spec.Source, err)
		}
		r.queue <- resolveResult{spec: spec, err: err}
	}()
}

func (r *Resolver) Finish() ([]Spec, error) {

	specs := make([]Spec, 0, r.jobs)
	errs := make([]string, 0, r.jobs)
	for r.jobs > 0 {
		result := <-r.queue
		r.jobs -= 1

		if result.err == nil {
			specs = append(specs, result.spec)
		} else {
			errs = append(errs, result.err.Error())
		}
	}

	if len(errs) > 0 {
		detail := strings.Join(errs, "; ")
		return specs, fmt.Errorf("failed to resolve container: %s", detail)
	}

	// Return a stable result, sorted by Digest
	sort.Slice(specs, func(i, j int) bool { return specs[i].Digest < specs[j].Digest })

	return specs, nil
}
