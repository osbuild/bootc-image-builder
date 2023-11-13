package blueprint

import (
	"fmt"
	"reflect"
	"strings"
)

type Customizations struct {
	Hostname           *string                   `json:"hostname,omitempty" toml:"hostname,omitempty"`
	Kernel             *KernelCustomization      `json:"kernel,omitempty" toml:"kernel,omitempty"`
	SSHKey             []SSHKeyCustomization     `json:"sshkey,omitempty" toml:"sshkey,omitempty"`
	User               []UserCustomization       `json:"user,omitempty" toml:"user,omitempty"`
	Group              []GroupCustomization      `json:"group,omitempty" toml:"group,omitempty"`
	Timezone           *TimezoneCustomization    `json:"timezone,omitempty" toml:"timezone,omitempty"`
	Locale             *LocaleCustomization      `json:"locale,omitempty" toml:"locale,omitempty"`
	Firewall           *FirewallCustomization    `json:"firewall,omitempty" toml:"firewall,omitempty"`
	Services           *ServicesCustomization    `json:"services,omitempty" toml:"services,omitempty"`
	Filesystem         []FilesystemCustomization `json:"filesystem,omitempty" toml:"filesystem,omitempty"`
	InstallationDevice string                    `json:"installation_device,omitempty" toml:"installation_device,omitempty"`
	FDO                *FDOCustomization         `json:"fdo,omitempty" toml:"fdo,omitempty"`
	OpenSCAP           *OpenSCAPCustomization    `json:"openscap,omitempty" toml:"openscap,omitempty"`
	Ignition           *IgnitionCustomization    `json:"ignition,omitempty" toml:"ignition,omitempty"`
	Directories        []DirectoryCustomization  `json:"directories,omitempty" toml:"directories,omitempty"`
	Files              []FileCustomization       `json:"files,omitempty" toml:"files,omitempty"`
	Repositories       []RepositoryCustomization `json:"repositories,omitempty" toml:"repositories,omitempty"`
}

type IgnitionCustomization struct {
	Embedded  *EmbeddedIgnitionCustomization  `json:"embedded,omitempty" toml:"embedded,omitempty"`
	FirstBoot *FirstBootIgnitionCustomization `json:"firstboot,omitempty" toml:"firstboot,omitempty"`
}

type EmbeddedIgnitionCustomization struct {
	Config string `json:"config,omitempty" toml:"config,omitempty"`
}

type FirstBootIgnitionCustomization struct {
	ProvisioningURL string `json:"url,omitempty" toml:"url,omitempty"`
}

type FDOCustomization struct {
	ManufacturingServerURL string `json:"manufacturing_server_url,omitempty" toml:"manufacturing_server_url,omitempty"`
	DiunPubKeyInsecure     string `json:"diun_pub_key_insecure,omitempty" toml:"diun_pub_key_insecure,omitempty"`
	// This is the output of:
	// echo "sha256:$(openssl x509 -fingerprint -sha256 -noout -in diun_cert.pem | cut -d"=" -f2 | sed 's/://g')"
	DiunPubKeyHash      string `json:"diun_pub_key_hash,omitempty" toml:"diun_pub_key_hash,omitempty"`
	DiunPubKeyRootCerts string `json:"diun_pub_key_root_certs,omitempty" toml:"diun_pub_key_root_certs,omitempty"`
}

type KernelCustomization struct {
	Name   string `json:"name,omitempty" toml:"name,omitempty"`
	Append string `json:"append" toml:"append"`
}

type SSHKeyCustomization struct {
	User string `json:"user" toml:"user"`
	Key  string `json:"key" toml:"key"`
}

type UserCustomization struct {
	Name        string   `json:"name" toml:"name"`
	Description *string  `json:"description,omitempty" toml:"description,omitempty"`
	Password    *string  `json:"password,omitempty" toml:"password,omitempty"`
	Key         *string  `json:"key,omitempty" toml:"key,omitempty"`
	Home        *string  `json:"home,omitempty" toml:"home,omitempty"`
	Shell       *string  `json:"shell,omitempty" toml:"shell,omitempty"`
	Groups      []string `json:"groups,omitempty" toml:"groups,omitempty"`
	UID         *int     `json:"uid,omitempty" toml:"uid,omitempty"`
	GID         *int     `json:"gid,omitempty" toml:"gid,omitempty"`
}

type GroupCustomization struct {
	Name string `json:"name" toml:"name"`
	GID  *int   `json:"gid,omitempty" toml:"gid,omitempty"`
}

type TimezoneCustomization struct {
	Timezone   *string  `json:"timezone,omitempty" toml:"timezone,omitempty"`
	NTPServers []string `json:"ntpservers,omitempty" toml:"ntpservers,omitempty"`
}

type LocaleCustomization struct {
	Languages []string `json:"languages,omitempty" toml:"languages,omitempty"`
	Keyboard  *string  `json:"keyboard,omitempty" toml:"keyboard,omitempty"`
}

type FirewallCustomization struct {
	Ports    []string                       `json:"ports,omitempty" toml:"ports,omitempty"`
	Services *FirewallServicesCustomization `json:"services,omitempty" toml:"services,omitempty"`
	Zones    []FirewallZoneCustomization    `json:"zones,omitempty" toml:"zones,omitempty"`
}

type FirewallZoneCustomization struct {
	Name    *string  `json:"name,omitempty" toml:"name,omitempty"`
	Sources []string `json:"sources,omitempty" toml:"sources,omitempty"`
}

type FirewallServicesCustomization struct {
	Enabled  []string `json:"enabled,omitempty" toml:"enabled,omitempty"`
	Disabled []string `json:"disabled,omitempty" toml:"disabled,omitempty"`
}

type ServicesCustomization struct {
	Enabled  []string `json:"enabled,omitempty" toml:"enabled,omitempty"`
	Disabled []string `json:"disabled,omitempty" toml:"disabled,omitempty"`
}

type OpenSCAPCustomization struct {
	DataStream string                           `json:"datastream,omitempty" toml:"datastream,omitempty"`
	ProfileID  string                           `json:"profile_id,omitempty" toml:"profile_id,omitempty"`
	Tailoring  *OpenSCAPTailoringCustomizations `json:"tailoring,omitempty" toml:"tailoring,omitempty"`
}

type OpenSCAPTailoringCustomizations struct {
	Selected   []string `json:"selected,omitempty" toml:"selected,omitempty"`
	Unselected []string `json:"unselected,omitempty" toml:"unselected,omitempty"`
}

type CustomizationError struct {
	Message string
}

func (e *CustomizationError) Error() string {
	return e.Message
}

// CheckCustomizations returns an error of type `CustomizationError`
// if `c` has any customizations not specified in `allowed`
func (c *Customizations) CheckAllowed(allowed ...string) error {
	if c == nil {
		return nil
	}

	allowMap := make(map[string]bool)

	for _, a := range allowed {
		allowMap[a] = true
	}

	t := reflect.TypeOf(*c)
	v := reflect.ValueOf(*c)

	for i := 0; i < t.NumField(); i++ {

		empty := false
		field := v.Field(i)

		switch field.Kind() {
		case reflect.String:
			if field.String() == "" {
				empty = true
			}
		case reflect.Array, reflect.Slice:
			if field.Len() == 0 {
				empty = true
			}
		case reflect.Ptr:
			if field.IsNil() {
				empty = true
			}
		default:
			panic(fmt.Sprintf("unhandled customization field type %s, %s", v.Kind(), t.Field(i).Name))

		}

		if !empty && !allowMap[t.Field(i).Name] {
			return &CustomizationError{fmt.Sprintf("'%s' is not allowed", t.Field(i).Name)}
		}
	}

	return nil
}

func (c *Customizations) GetHostname() *string {
	if c == nil {
		return nil
	}
	return c.Hostname
}

func (c *Customizations) GetPrimaryLocale() (*string, *string) {
	if c == nil {
		return nil, nil
	}
	if c.Locale == nil {
		return nil, nil
	}
	if len(c.Locale.Languages) == 0 {
		return nil, c.Locale.Keyboard
	}
	return &c.Locale.Languages[0], c.Locale.Keyboard
}

func (c *Customizations) GetTimezoneSettings() (*string, []string) {
	if c == nil {
		return nil, nil
	}
	if c.Timezone == nil {
		return nil, nil
	}
	return c.Timezone.Timezone, c.Timezone.NTPServers
}

func (c *Customizations) GetUsers() []UserCustomization {
	if c == nil {
		return nil
	}

	users := []UserCustomization{}

	// prepend sshkey for backwards compat (overridden by users)
	if len(c.SSHKey) > 0 {
		for idx := range c.SSHKey {
			keyc := c.SSHKey[idx]
			users = append(users, UserCustomization{
				Name: keyc.User,
				Key:  &keyc.Key,
			})
		}
	}

	users = append(users, c.User...)

	// sanitize user home directory in blueprint: if it has a trailing slash,
	// it might lead to the directory not getting the correct selinux labels
	for idx := range users {
		u := users[idx]
		if u.Home != nil {
			homedir := strings.TrimRight(*u.Home, "/")
			u.Home = &homedir
			users[idx] = u
		}
	}
	return users
}

func (c *Customizations) GetGroups() []GroupCustomization {
	if c == nil {
		return nil
	}

	return c.Group
}

func (c *Customizations) GetKernel() *KernelCustomization {
	var name string
	var append string
	if c != nil && c.Kernel != nil {
		name = c.Kernel.Name
		append = c.Kernel.Append
	}

	if name == "" {
		name = "kernel"
	}

	return &KernelCustomization{
		Name:   name,
		Append: append,
	}
}

func (c *Customizations) GetFirewall() *FirewallCustomization {
	if c == nil {
		return nil
	}

	return c.Firewall
}

func (c *Customizations) GetServices() *ServicesCustomization {
	if c == nil {
		return nil
	}

	return c.Services
}

func (c *Customizations) GetFilesystems() []FilesystemCustomization {
	if c == nil {
		return nil
	}
	return c.Filesystem
}

func (c *Customizations) GetFilesystemsMinSize() uint64 {
	if c == nil {
		return 0
	}
	var agg uint64
	for _, m := range c.Filesystem {
		agg += m.MinSize
	}
	// This ensures that file system customization `size` is a multiple of
	// sector size (512)
	if agg%512 != 0 {
		agg = (agg/512 + 1) * 512
	}
	return agg
}

func (c *Customizations) GetInstallationDevice() string {
	if c == nil || c.InstallationDevice == "" {
		return ""
	}
	return c.InstallationDevice
}

func (c *Customizations) GetFDO() *FDOCustomization {
	if c == nil {
		return nil
	}
	return c.FDO
}

func (c *Customizations) GetOpenSCAP() *OpenSCAPCustomization {
	if c == nil {
		return nil
	}
	return c.OpenSCAP
}

func (c *Customizations) GetIgnition() *IgnitionCustomization {
	if c == nil {
		return nil
	}
	return c.Ignition
}

func (c *Customizations) GetDirectories() []DirectoryCustomization {
	if c == nil {
		return nil
	}
	return c.Directories
}

func (c *Customizations) GetFiles() []FileCustomization {
	if c == nil {
		return nil
	}
	return c.Files
}

func (c *Customizations) GetRepositories() ([]RepositoryCustomization, error) {
	if c == nil {
		return nil, nil
	}

	for idx := range c.Repositories {
		err := validateCustomRepository(&c.Repositories[idx])
		if err != nil {
			return nil, err
		}
	}

	return c.Repositories, nil
}
