package blueprint

import (
	"encoding/json"
	"fmt"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/pathpolicy"
)

type FilesystemCustomization struct {
	Mountpoint string `json:"mountpoint,omitempty" toml:"mountpoint,omitempty"`
	MinSize    uint64 `json:"minsize,omitempty" toml:"size,omitempty"`
}

func (fsc *FilesystemCustomization) UnmarshalTOML(data interface{}) error {
	d, _ := data.(map[string]interface{})

	switch d["mountpoint"].(type) {
	case string:
		fsc.Mountpoint = d["mountpoint"].(string)
	default:
		return fmt.Errorf("TOML unmarshal: mountpoint must be string, got %v of type %T", d["mountpoint"], d["mountpoint"])
	}

	switch d["size"].(type) {
	case int64:
		fsc.MinSize = uint64(d["size"].(int64))
	case string:
		size, err := common.DataSizeToUint64(d["size"].(string))
		if err != nil {
			return fmt.Errorf("TOML unmarshal: size is not valid filesystem size (%w)", err)
		}
		fsc.MinSize = size
	default:
		return fmt.Errorf("TOML unmarshal: size must be integer or string, got %v of type %T", d["size"], d["size"])
	}

	return nil
}

func (fsc *FilesystemCustomization) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	d, _ := v.(map[string]interface{})

	switch d["mountpoint"].(type) {
	case string:
		fsc.Mountpoint = d["mountpoint"].(string)
	default:
		return fmt.Errorf("JSON unmarshal: mountpoint must be string, got %v of type %T", d["mountpoint"], d["mountpoint"])
	}

	// The JSON specification only mentions float64 and Go defaults to it: https://go.dev/blog/json
	switch d["minsize"].(type) {
	case float64:
		// Note that it uses different key than the TOML version
		fsc.MinSize = uint64(d["minsize"].(float64))
	case string:
		size, err := common.DataSizeToUint64(d["minsize"].(string))
		if err != nil {
			return fmt.Errorf("JSON unmarshal: size is not valid filesystem size (%w)", err)
		}
		fsc.MinSize = size
	default:
		return fmt.Errorf("JSON unmarshal: minsize must be float64 number or string, got %v of type %T", d["minsize"], d["minsize"])
	}

	return nil
}

// CheckMountpointsPolicy checks if the mountpoints are allowed by the policy
func CheckMountpointsPolicy(mountpoints []FilesystemCustomization, mountpointAllowList *pathpolicy.PathPolicies) error {
	invalidMountpoints := []string{}
	for _, m := range mountpoints {
		err := mountpointAllowList.Check(m.Mountpoint)
		if err != nil {
			invalidMountpoints = append(invalidMountpoints, m.Mountpoint)
		}
	}

	if len(invalidMountpoints) > 0 {
		return fmt.Errorf("The following custom mountpoints are not supported %+q", invalidMountpoints)
	}

	return nil
}
