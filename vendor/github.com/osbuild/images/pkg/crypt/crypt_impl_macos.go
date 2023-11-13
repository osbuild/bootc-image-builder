//go:build darwin

package crypt

func crypt(pass, salt string) (string, error) {
	panic("You must not run osbuild-composer on macOS!")
}
