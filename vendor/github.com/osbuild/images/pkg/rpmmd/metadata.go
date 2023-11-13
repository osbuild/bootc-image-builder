package rpmmd

import (
	"fmt"
)

type RPM struct {
	Type      string  `json:"type"` // must be 'rpm'
	Name      string  `json:"name"`
	Version   string  `json:"version"`
	Release   string  `json:"release"`
	Epoch     *string `json:"epoch,omitempty"`
	Arch      string  `json:"arch"`
	Sigmd5    string  `json:"sigmd5"`
	Signature *string `json:"signature"`
}

// NEVRA string for the package
func (r RPM) String() string {
	epoch := ""
	if r.Epoch != nil {
		epoch = *r.Epoch + ":"
	}
	return fmt.Sprintf("%s-%s%s-%s.%s", r.Name, epoch, r.Version, r.Release, r.Arch)
}

// Deduplicate a list of RPMs based on NEVRA string
func DeduplicateRPMs(rpms []RPM) []RPM {
	rpmMap := make(map[string]struct{}, len(rpms))
	uniqueRPMs := make([]RPM, 0, len(rpms))

	for _, rpm := range rpms {
		if _, added := rpmMap[rpm.String()]; !added {
			rpmMap[rpm.String()] = struct{}{}
			uniqueRPMs = append(uniqueRPMs, rpm)
		}
	}
	return uniqueRPMs
}
