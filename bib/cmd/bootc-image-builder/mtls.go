package main

import (
	"fmt"
	"os"
	"path"

	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/sirupsen/logrus"
)

type mTLSConfig struct {
	key  []byte
	cert []byte
	ca   []byte
}

var osReadFile = os.ReadFile

func extractTLSKeys(repoSets map[string][]rpmmd.RepoConfig) (*mTLSConfig, error) {
	var keyPath, certPath, caPath string
	for _, set := range repoSets {
		for _, r := range set {
			if r.SSLClientKey != "" {
				if keyPath != "" && (keyPath != r.SSLClientKey || certPath != r.SSLClientCert || caPath != r.SSLCACert) {
					return nil, fmt.Errorf("multiple TLS client keys found, this is currently unsupported")
				}

				keyPath = r.SSLClientKey
				certPath = r.SSLClientCert
				caPath = r.SSLCACert
			}
		}
	}
	if keyPath == "" {
		return nil, nil
	}

	key, err := osReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read TLS client key from the container: %w", err)
	}

	cert, err := osReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read TLS client certificate from the container: %w", err)
	}

	ca, err := osReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read TLS CA certificate from the container: %w", err)
	}

	return &mTLSConfig{
		key:  key,
		cert: cert,
		ca:   ca,
	}, nil
}

// prepareOsbuildMTLSConfig writes the given mTLS keys to the given directory and returns the environment variables
// to set for osbuild
func prepareOsbuildMTLSConfig(mTLS *mTLSConfig) (envVars []string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "osbuild-mtls")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temporary directory for osbuild mTLS keys: %w", err)
	}

	cleanupFn := func() {
		if err := os.RemoveAll(dir); err != nil {
			logrus.Warnf("prepareOsbuildMTLSConfig: failed to remove temporary directory %s: %v", dir, err)
		}
	}

	defer func() {
		if err != nil {
			cleanupFn()
		}
	}()

	keyPath := path.Join(dir, "client.key")
	certPath := path.Join(dir, "client.crt")
	caPath := path.Join(dir, "ca.crt")
	if err := os.WriteFile(keyPath, mTLS.key, 0600); err != nil {
		return nil, nil, fmt.Errorf("failed to write TLS client key for osbuild: %w", err)
	}
	if err := os.WriteFile(certPath, mTLS.cert, 0600); err != nil {
		return nil, nil, fmt.Errorf("failed to write TLS client certificate for osbuild: %w", err)
	}
	if err := os.WriteFile(caPath, mTLS.ca, 0644); err != nil {
		return nil, nil, fmt.Errorf("failed to write TLS CA certificate for osbuild: %w", err)
	}

	return []string{
		fmt.Sprintf("OSBUILD_SOURCES_CURL_SSL_CLIENT_KEY=%s", keyPath),
		fmt.Sprintf("OSBUILD_SOURCES_CURL_SSL_CLIENT_CERT=%s", certPath),
		fmt.Sprintf("OSBUILD_SOURCES_CURL_SSL_CA_CERT=%s", caPath),
	}, cleanupFn, nil
}
