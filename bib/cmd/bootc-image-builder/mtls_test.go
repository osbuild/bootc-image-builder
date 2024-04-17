package main

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeFileReader struct {
	readPaths []string
}

func (f *fakeFileReader) ReadFile(path string) ([]byte, error) {
	f.readPaths = append(f.readPaths, path)
	return []byte(fmt.Sprintf("content of %s", path)), nil
}

func TestExtractTLSKeysHappy(t *testing.T) {
	repos := map[string][]rpmmd.RepoConfig{
		"kingfisher": {
			{
				SSLCACert:     "/ca",
				SSLClientCert: "/cert",
				SSLClientKey:  "/key",
			},
		},
	}

	fakeReader := &fakeFileReader{}

	mTLS, err := extractTLSKeys(fakeReader, repos)
	require.NoError(t, err)
	require.Equal(t, mTLS.ca, []byte("content of /ca"))
	require.Equal(t, mTLS.cert, []byte("content of /cert"))
	require.Equal(t, mTLS.key, []byte("content of /key"))
	require.Len(t, fakeReader.readPaths, 3)

	// also check that adding another repo with same keys still succeeds
	repos["toucan"] = repos["kingfisher"]
	_, err = extractTLSKeys(fakeReader, repos)
	require.NoError(t, err)
	require.Len(t, fakeReader.readPaths, 6)
}

func TestExtractTLSKeysUnhappy(t *testing.T) {
	repos := map[string][]rpmmd.RepoConfig{
		"kingfisher": {
			{
				SSLCACert:     "/ca",
				SSLClientCert: "/cert",
				SSLClientKey:  "/key",
			},
		},

		"vulture": {
			{
				SSLCACert:     "/different-ca",
				SSLClientCert: "/different-cert",
				SSLClientKey:  "/different-key",
			},
		},
	}

	fakeReader := &fakeFileReader{}

	_, err := extractTLSKeys(fakeReader, repos)
	require.EqualError(t, err, "multiple TLS client keys found, this is currently unsupported")
}

func TestPrepareOsbuildMTLSConfig(t *testing.T) {
	mTLS := mTLSConfig{
		key:  []byte("key"),
		cert: []byte("cert"),
		ca:   []byte("ca"),
	}

	envVars, cleanup, err := prepareOsbuildMTLSConfig(&mTLS)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	require.Len(t, envVars, 3)

	validateVar := func(envVar, content string) {
		for _, e := range envVars {
			if strings.HasPrefix(e, envVar+"=") {
				envVarParts := strings.SplitN(e, "=", 2)
				assert.Len(t, envVarParts, 2)

				actualContent, err := os.ReadFile(envVarParts[1])
				assert.NoError(t, err)

				assert.Equal(t, content, string(actualContent))
				return
			}

		}

		assert.Fail(t, "environment variable not found", "%s", envVar)
	}

	validateVar("OSBUILD_SOURCES_CURL_SSL_CLIENT_KEY", "key")
	validateVar("OSBUILD_SOURCES_CURL_SSL_CLIENT_CERT", "cert")
	validateVar("OSBUILD_SOURCES_CURL_SSL_CA_CERT", "ca")
}

func TestPrepareOsbuildMTLSConfigCleanup(t *testing.T) {
	mTLS := mTLSConfig{
		key:  []byte("key"),
		cert: []byte("cert"),
		ca:   []byte("ca"),
	}

	envVars, cleanup, err := prepareOsbuildMTLSConfig(&mTLS)
	require.NoError(t, err)

	// quick and dirty way to get the temporary directory
	filepath := strings.SplitN(envVars[0], "=", 2)[1]
	tmpdir := path.Dir(filepath)

	// check that the cleanup works
	assert.DirExists(t, tmpdir)
	cleanup()
	assert.NoDirExists(t, tmpdir)
}
