package osbuild

// Expose a file (or part of it) as a device node

type LoopbackDeviceOptions struct {
	// File to associate with the loopback device
	Filename string `json:"filename"`

	// Start of the data segment
	Start uint64 `json:"start,omitempty"`

	// Size limit of the data segment (in sectors)
	Size uint64 `json:"size,omitempty"`

	// Sector size (in bytes)
	SectorSize *uint64 `json:"sector-size,omitempty"`

	// Lock (bsd lock) the device after opening it
	Lock bool `json:"lock,omitempty"`
}

func (LoopbackDeviceOptions) isDeviceOptions() {}

func NewLoopbackDevice(options *LoopbackDeviceOptions) *Device {
	return &Device{
		Type:    "org.osbuild.loopback",
		Options: options,
	}
}
