// Copyright 2020 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nproc

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// GetProcessors returns a count for number of cores we should use;
// this value is appropriate to pass to e.g. make -J as well as
// qemu -smp for example.
func GetProcessors() (uint, error) {
	// Get available CPU count, including sched_getaffinity()
	nprocBuf, err := exec.Command("nproc").CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("executing nproc: %w", err)
	}
	nproc, err := strconv.ParseUint(strings.TrimSpace(string(nprocBuf)), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parsing nproc output: %w", err)
	}

	// Compute the available CPU quota
	quota, err := getCpuQuota()
	if err != nil {
		return 0, err
	}

	if quota < uint(nproc) {
		return quota, nil
	}
	return uint(nproc), nil
}

func getCpuQuota() (uint, error) {
	// cgroups v2
	buf, err := os.ReadFile("/sys/fs/cgroup/cpu.max")
	if err == nil {
		vals := strings.SplitN(strings.TrimSpace(string(buf)), " ", 2)
		if len(vals) != 2 {
			return 0, fmt.Errorf("invalid cpu.max value")
		}
		if vals[0] != "max" {
			quota, err := strconv.ParseUint(vals[0], 10, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid CPU quota: %w", err)
			}
			period, err := strconv.ParseUint(vals[1], 10, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid CPU period: %w", err)
			}
			if quota > 0 && period > 0 {
				return uint((quota + period - 1) / period), nil
			}
		}
	} else if !os.IsNotExist(err) {
		return 0, fmt.Errorf("reading cpu.max: %w", err)
	}

	// cgroups v1
	buf, err = os.ReadFile("/sys/fs/cgroup/cpu/cpu.cfs_quota_us")
	if os.IsNotExist(err) {
		return math.MaxUint, nil
	} else if err != nil {
		return 0, fmt.Errorf("reading cpu.cfs_quota_us: %w", err)
	}
	// can be -1
	quota, err := strconv.ParseInt(strings.TrimSpace(string(buf)), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU quota: %w", err)
	}
	buf, err = os.ReadFile("/sys/fs/cgroup/cpu/cpu.cfs_period_us")
	if os.IsNotExist(err) {
		return math.MaxUint, nil
	} else if err != nil {
		return 0, fmt.Errorf("reading cpu.cfs_period_us: %w", err)
	}
	period, err := strconv.ParseUint(strings.TrimSpace(string(buf)), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU period: %w", err)
	}
	if quota > 0 && period > 0 {
		return uint((uint64(quota) + period - 1) / period), nil
	}

	return math.MaxUint, nil
}
