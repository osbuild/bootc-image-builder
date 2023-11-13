package rhsm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/ini.v1"
)

type subscription struct {
	id            string
	baseurl       string
	sslCACert     string
	sslClientKey  string
	sslClientCert string
}

// Subscriptions encapsulates all available subscriptions from the
// host system.
type Subscriptions struct {
	available []subscription
	secrets   *RHSMSecrets // secrets are used in there is no matching subscription

	Consumer *ConsumerSecrets
}

// RHSMSecrets represents a set of CA certificate, client key, and
// client certificate for a specific repository.
type RHSMSecrets struct {
	SSLCACert     string
	SSLClientKey  string
	SSLClientCert string
}

// These secrets are present on any subscribed system and uniquely identify the host
type ConsumerSecrets struct {
	ConsumerKey  string
	ConsumerCert string
}

func getRHSMSecrets() (*RHSMSecrets, error) {
	keys, err := filepath.Glob("/etc/pki/entitlement/*-key.pem")
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		cert := strings.TrimSuffix(key, "-key.pem") + ".pem"
		if _, err := os.Stat(cert); err == nil {
			return &RHSMSecrets{
				SSLCACert:     "/etc/rhsm/ca/redhat-uep.pem",
				SSLClientKey:  key,
				SSLClientCert: cert,
			}, nil
		}
	}
	return nil, fmt.Errorf("no matching key and certificate pair")
}

func getListOfSubscriptions() ([]subscription, error) {
	// This file has a standard syntax for yum repositories which is
	// documented in `man yum.conf`. The same parsing mechanism could
	// be used for any other repo file in /etc/yum.repos.d/.
	availableSubscriptionsFile := "/etc/yum.repos.d/redhat.repo"
	content, err := os.ReadFile(availableSubscriptionsFile)
	if err != nil {
		if pErr, ok := err.(*os.PathError); ok {
			if pErr.Err.Error() == "no such file or directory" {
				// The system is not subscribed
				return nil, nil
			}
		}
		return nil, fmt.Errorf("failed to open the file with subscriptions: %w", err)
	}
	subscriptions, err := parseRepoFile(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the file with subscriptions: %w", err)
	}

	return subscriptions, nil
}

func getConsumerSecrets() (*ConsumerSecrets, error) {
	res := ConsumerSecrets{
		ConsumerKey:  "/etc/pki/consumer/key.pem",
		ConsumerCert: "/etc/pki/consumer/cert.pem",
	}

	if _, err := os.Stat(res.ConsumerKey); err != nil {
		return nil, fmt.Errorf("no consumer key found")
	}
	if _, err := os.Stat(res.ConsumerCert); err != nil {
		return nil, fmt.Errorf("no consumer cert found")
	}
	return &res, nil
}

// LoadSystemSubscriptions loads all the available subscriptions.
func LoadSystemSubscriptions() (*Subscriptions, error) {
	consumerSecrets, err := getConsumerSecrets()
	if err != nil {
		logrus.Warnf("Failed to load consumer certs: %v", err)
	}

	subscriptions, err1 := getListOfSubscriptions()
	secrets, err2 := getRHSMSecrets()
	if subscriptions == nil && secrets == nil {
		// Neither works, return an error because at least one has to be available
		if err1 != nil {
			return nil, err1
		}
		if err2 != nil {
			return nil, err2
		}
		return nil, fmt.Errorf("failed to load subscriptions")
	}

	return &Subscriptions{
		available: subscriptions,
		secrets:   secrets,

		Consumer: consumerSecrets,
	}, nil
}

func parseRepoFile(content []byte) ([]subscription, error) {
	cfg, err := ini.Load(content)
	if err != nil {
		return nil, err
	}

	subscriptions := make([]subscription, 0)

	for _, section := range cfg.Sections() {
		id := section.Name()
		key, err := section.GetKey("baseurl")
		if err != nil {
			continue
		}
		baseurl := key.String()
		key, err = section.GetKey("sslcacert")
		if err != nil {
			continue
		}
		sslcacert := key.String()
		key, err = section.GetKey("sslclientkey")
		if err != nil {
			continue
		}
		sslclientkey := key.String()
		key, err = section.GetKey("sslclientcert")
		if err != nil {
			continue
		}
		sslclientcert := key.String()
		subscriptions = append(subscriptions, subscription{
			id:            id,
			baseurl:       baseurl,
			sslCACert:     sslcacert,
			sslClientKey:  sslclientkey,
			sslClientCert: sslclientcert,
		})
	}

	return subscriptions, nil
}

// GetSecretsForBaseurl queries the Subscriptions structure for a RHSMSecrets of a single repository.
func (s *Subscriptions) GetSecretsForBaseurl(baseurls []string, arch, releasever string) (*RHSMSecrets, error) {
	for _, subs := range s.available {
		for _, baseurl := range baseurls {
			url := strings.Replace(subs.baseurl, "$basearch", arch, -1)
			url = strings.Replace(url, "$releasever", releasever, -1)
			if url == baseurl {
				return &RHSMSecrets{
					SSLCACert:     subs.sslCACert,
					SSLClientKey:  subs.sslClientKey,
					SSLClientCert: subs.sslClientCert,
				}, nil
			}
		}
	}
	// If there is no matching URL, fall back to the global secrets
	if s.secrets != nil {
		return s.secrets, nil
	}
	return nil, fmt.Errorf("no such baseurl in the available subscriptions")
}
