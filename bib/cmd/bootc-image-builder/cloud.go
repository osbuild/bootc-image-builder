package main

import (
	"fmt"
	"io"
	"os"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/pflag"

	"github.com/osbuild/images/pkg/cloud"
)

func upload(uploader cloud.Uploader, path string, flags *pflag.FlagSet) error {
	progress, err := flags.GetString("progress")
	if err != nil {
		return err
	}

	// TODO: extract this as a helper once we add "uploadAzure" or
	// similar. Eventually we may provide json progress here too.
	var pbar *pb.ProgressBar
	switch progress {
	case "auto", "verbose", "term":
		pbar = pb.New(0)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot upload: %v", err)
	}
	// nolint:errcheck
	defer file.Close()

	var r io.Reader = file
	if pbar != nil {
		st, err := file.Stat()
		if err != nil {
			return err
		}
		pbar.SetTotal(st.Size())
		pbar.Set(pb.Bytes, true)
		pbar.SetWriter(osStdout)
		r = pbar.NewProxyReader(file)
		pbar.Start()
		defer pbar.Finish()
	}

	return uploader.UploadAndRegister(r, osStderr)
}
