package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening %s: %s", src, err.Error())
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("error creating %s: %s", dest, err.Error())
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("error copying %s -> %s: %s", src, dest, err.Error())
	}
	return nil
}

func copyDir(src, dest string) error {
	// convert paths to absolutes
	if absSrc, err := filepath.Abs(src); err != nil {
		return err
	} else {
		src = absSrc
	}
	if absDest, err := filepath.Abs(dest); err != nil {
		return err
	} else {
		dest = absDest
	}

	// create dest
	if err := os.MkdirAll(dest, os.ModePerm); err != nil {
		return fmt.Errorf("error creating destination directory %s: %s", dest, err.Error())
	}

	err := filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		pathTail, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		fileDest := filepath.Join(dest, pathTail)

		if info.IsDir() {
			err := os.MkdirAll(fileDest, os.ModePerm)
			if err != nil {
				return fmt.Errorf("error creating directory %s: %s", fileDest, err.Error())
			}
			return nil
		}
		return copyFile(path, fileDest)
	})
	return err
}
