package main

import (
	"github.com/osbuild/images/pkg/cloud"
	"github.com/osbuild/images/pkg/cloud/awscloud"
)

var (
	CanChownInPath    = canChownInPath
	CreateRand        = createRand
	BuildCobraCmdline = buildCobraCmdline
	HandleAWSFlags    = handleAWSFlags
)

func MockOsGetuid(new func() int) (restore func()) {
	saved := osGetuid
	osGetuid = new
	return func() {
		osGetuid = saved
	}
}

func MockOsReadFile(new func(string) ([]byte, error)) (restore func()) {
	saved := osReadFile
	osReadFile = new
	return func() {
		osReadFile = saved
	}
}

func MockAwscloudNewUploader(f func(string, string, string, *awscloud.UploaderOptions) (cloud.Uploader, error)) (restore func()) {
	saved := awscloudNewUploader
	awscloudNewUploader = f
	return func() {
		awscloudNewUploader = saved
	}
}
