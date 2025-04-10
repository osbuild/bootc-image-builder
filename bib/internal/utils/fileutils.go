package utils

import (
	"os"

	"github.com/sirupsen/logrus"
)

// LogClose closes the file and logs any error encountered during the operation.
func LogClose(file *os.File) {
	if err := file.Close(); err != nil {
		logrus.WithError(err).Errorf("failed to close file")
	}
}
