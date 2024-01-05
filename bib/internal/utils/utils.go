package utils

import "os/exec"

// IsMountpoint checks if the target path is a mount point
func IsMountpoint(path string) bool {
	c := exec.Command("mountpoint", path)
	c.Stderr = nil
	c.Stdout = nil
	return c.Run() == nil
}
