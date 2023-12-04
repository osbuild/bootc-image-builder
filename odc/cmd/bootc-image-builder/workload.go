package main

import "github.com/osbuild/images/pkg/rpmmd"

// NullWorkload implements the images Workload interface but returns only nil
// from all its methods and holds no data.
type NullWorkload struct {
}

func (p *NullWorkload) GetRepos() []rpmmd.RepoConfig {
	return nil
}

func (p *NullWorkload) GetPackages() []string {
	return nil
}

func (p *NullWorkload) GetServices() []string {
	return nil
}

func (p *NullWorkload) GetDisabledServices() []string {
	return nil
}
