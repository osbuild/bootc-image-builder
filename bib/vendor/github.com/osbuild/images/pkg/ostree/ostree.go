package ostree

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/osbuild/images/pkg/rhsm"
)

var (
	ostreeRefRE    = regexp.MustCompile(`^(?:[\w\d][-._\w\d]*\/)*[\w\d][-._\w\d]*$`)
	ostreeCommitRE = regexp.MustCompile("^[0-9a-f]{64}$")
)

// SourceSpec serves as input for ResolveParams, and contains all necessary
// variables to resolve a ref, which can then be turned into a CommitSpec.
type SourceSpec struct {
	URL  string
	Ref  string
	RHSM bool
}

// CommitSpec specifies an ostree commit using any combination of Ref (branch), URL (source), and Checksum (commit ID).
type CommitSpec struct {

	// Ref for the commit. Can be empty.
	Ref string

	// URL of the repo where the commit can be fetched, if available.
	URL string

	ContentURL string

	Secrets string

	// Checksum of the commit.
	Checksum string
}

// ImageOptions specify an ostree ref, checksum, URL, ContentURL, and RHSM. The
// meaning of each parameter depends on the image type being built. The type
// is used to specify ostree-related image options when initializing a
// Manifest.
type ImageOptions struct {
	// For ostree commit and container types: The ref of the new commit to be
	// built.
	// For ostree installers and raw images: The ref of the commit being
	// embedded in the installer or deployed in the image.
	ImageRef string `json:"ref"`

	// For ostree commit and container types: The ParentRef specifies the parent
	// ostree commit that the new commit will be based on.
	// For ostree installers and raw images: The ParentRef does not apply.
	ParentRef string `json:"parent"`

	// The URL from which to fetch the commit specified by the checksum.
	URL string `json:"url"`

	// If specified, the URL will be used only for metadata.
	ContentURL string `json:"contenturl"`

	// Indicate if the 'org.osbuild.rhsm.consumer' secret should be added when pulling from the
	// remote.
	RHSM bool `json:"rhsm"`
}

// Validate the image options. This doesn't verify the existence of any remote
// objects and does not guarantee that refs will be successfully resolved. It
// only checks that the values and value combinations are valid.
//
// The function checks the following:
// - The ImageRef, if specified, is a valid ref and does not look like a
// checksum.
// - The ParentRef, if specified, must be a valid ref or a checksum.
// - If the ParentRef is specified, the URL must also be specified.
// - URLs must be valid.
func (options ImageOptions) Validate() error {
	if ref := options.ImageRef; ref != "" {
		// image ref must not look like a checksum
		if verifyChecksum(ref) {
			return NewRefError("ostree image ref looks like a checksum %q", ref)
		}
		if !verifyRef(ref) {
			return NewRefError("invalid ostree image ref %q", ref)
		}
	}

	if parent := options.ParentRef; parent != "" {
		if !verifyChecksum(parent) && !verifyRef(parent) {
			return NewRefError("invalid ostree parent ref or commit %q", parent)
		}

		// valid URL required
		if purl := options.URL; purl == "" {
			return NewParameterComboError("ostree parent ref specified, but no URL to retrieve it")
		}
	}

	// whether required or not, any URL specified must be valid
	if purl := options.URL; purl != "" {
		if _, err := url.ParseRequestURI(purl); err != nil {
			return fmt.Errorf("ostree URL %q is invalid", purl)
		}
	}

	if curl := options.ContentURL; curl != "" {
		if _, err := url.ParseRequestURI(curl); err != nil {
			return fmt.Errorf("ostree content URL %q is invalid", curl)
		}
	}

	return nil
}

// Remote defines the options that can be set for an OSTree Remote configuration.
type Remote struct {
	Name        string
	URL         string
	ContentURL  string
	GPGKeyPaths []string
}

func verifyRef(ref string) bool {
	return len(ref) > 0 && ostreeRefRE.MatchString(ref)
}

func verifyChecksum(commit string) bool {
	return len(commit) > 0 && ostreeCommitRE.MatchString(commit)
}

// ResolveRef resolves the URL path specified by the location and ref
// (location+"refs/heads/"+ref) and returns the commit ID for the named ref. If
// there is an error, it will be of type ResolveRefError.
func ResolveRef(location, ref string, consumerCerts bool, subs *rhsm.Subscriptions, ca *string) (string, error) {
	u, err := url.Parse(location)
	if err != nil {
		return "", NewResolveRefError(fmt.Sprintf("error parsing ostree repository location: %v", err))
	}
	u.Path = path.Join(u.Path, "refs/heads/", ref)

	var client *http.Client
	if consumerCerts {
		if subs == nil {
			subs, err = rhsm.LoadSystemSubscriptions()
			if subs.Consumer == nil || err != nil {
				return "", NewResolveRefError("error adding rhsm certificates when resolving ref")
			}
		}

		tlsConf := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		if ca != nil {
			caCertPEM, err := os.ReadFile(*ca)
			if err != nil {
				return "", NewResolveRefError("error adding rhsm certificates when resolving ref: %s", err)
			}
			roots := x509.NewCertPool()
			ok := roots.AppendCertsFromPEM(caCertPEM)
			if !ok {
				return "", NewResolveRefError("error adding rhsm certificates when resolving ref")
			}
			tlsConf.RootCAs = roots
		}

		cert, err := tls.LoadX509KeyPair(subs.Consumer.ConsumerCert, subs.Consumer.ConsumerKey)
		if err != nil {
			return "", NewResolveRefError("error adding rhsm certificates when resolving ref: %s", err)
		}
		tlsConf.Certificates = []tls.Certificate{cert}

		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConf,
			},
			Timeout: 300 * time.Second,
		}
	} else {
		client = &http.Client{}
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", NewResolveRefError("error preparing ostree resolve request: %s", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", NewResolveRefError(fmt.Sprintf("error sending request to ostree repository %q: %v", u.String(), err))
	}
	if resp.StatusCode != http.StatusOK {
		return "", NewResolveRefError("ostree repository %q returned status: %s", u.String(), resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", NewResolveRefError(fmt.Sprintf("error reading response from ostree repository %q: %v", u.String(), err))
	}
	checksum := strings.TrimSpace(string(body))
	// Check that this is at least a hex string.
	_, err = hex.DecodeString(checksum)
	if err != nil {
		return "", NewResolveRefError("ostree repository %q returned invalid reference", u.String())
	}
	return checksum, nil
}

// Resolve the ostree source specification to a commit specification.
//
// If a URL is defined in the source specification, the checksum of the ref is
// resolved, otherwise the checksum is an empty string. Failure to resolve the
// checksum results in a ResolveRefError.
//
// If the ref is already a checksum (64 alphanumeric characters), it is not
// resolved or checked against the repository.
//
// If the ref is malformed, the function returns with a RefError.
func Resolve(source SourceSpec) (CommitSpec, error) {
	commit := CommitSpec{
		Ref: source.Ref,
		URL: source.URL,
	}

	if source.RHSM {
		commit.Secrets = "org.osbuild.rhsm.consumer"
	}

	if verifyChecksum(source.Ref) {
		// the ref is a commit: return as is
		commit.Checksum = source.Ref
		return commit, nil
	}

	if !verifyRef(source.Ref) {
		// the ref is not a commit and it's also an invalid ref
		return CommitSpec{}, NewRefError("Invalid ostree ref or commit %q", source.Ref)
	}

	// URL set: Resolve checksum
	if source.URL != "" {
		// If a URL is specified, we need to fetch the commit at the URL.
		checksum, err := ResolveRef(source.URL, source.Ref, source.RHSM, nil, nil)
		if err != nil {
			return CommitSpec{}, err // ResolveRefError
		}
		commit.Checksum = checksum
	}
	return commit, nil
}
