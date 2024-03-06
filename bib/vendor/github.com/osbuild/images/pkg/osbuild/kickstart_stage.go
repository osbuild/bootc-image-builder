package osbuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/users"
)

const (
	KickstartPathInteractiveDefaults = "/usr/share/anaconda/interactive-defaults.ks"
	KickstartPathOSBuild             = "/osbuild.ks"
)

type KickstartStageOptions struct {
	// Where to place the kickstart file
	Path string `json:"path"`

	OSTreeCommit    *OSTreeCommitOptions    `json:"ostree,omitempty"`
	OSTreeContainer *OSTreeContainerOptions `json:"ostreecontainer,omitempty"`

	LiveIMG *LiveIMGOptions `json:"liveimg,omitempty"`

	Users map[string]UsersStageOptionsUser `json:"users,omitempty"`

	Groups map[string]GroupsStageOptionsGroup `json:"groups,omitempty"`

	Lang         string               `json:"lang,omitempty"`
	Keyboard     string               `json:"keyboard,omitempty"`
	Timezone     string               `json:"timezone,omitempty"`
	DisplayMode  string               `json:"display_mode,omitempty"`
	Reboot       *RebootOptions       `json:"reboot,omitempty"`
	RootPassword *RootPasswordOptions `json:"rootpw,omitempty"`
	ZeroMBR      bool                 `json:"zerombr,omitempty"`
	ClearPart    *ClearPartOptions    `json:"clearpart,omitempty"`
	AutoPart     *AutoPartOptions     `json:"autopart,omitempty"`
	Network      []NetworkOptions     `json:"network,omitempty"`
}

type LiveIMGOptions struct {
	URL string `json:"url"`
}

type OSTreeCommitOptions struct {
	OSName string `json:"osname"`
	Remote string `json:"remote"`
	URL    string `json:"url"`
	Ref    string `json:"ref"`
	GPG    bool   `json:"gpg"`
}

type OSTreeContainerOptions struct {
	StateRoot             string `json:"stateroot"`
	URL                   string `json:"url"`
	Transport             string `json:"transport"`
	Remote                string `json:"remote"`
	SignatureVerification bool   `json:"signatureverification"`
}

type RebootOptions struct {
	Eject bool `json:"eject,omitempty"`
	KExec bool `json:"kexec,omitempty"`
}

type ClearPartOptions struct {
	All       bool     `json:"all,omitempty"`
	InitLabel bool     `json:"initlabel,omitempty"`
	Drives    []string `json:"drives,omitempty"`
	List      []string `json:"list,omitempty"`
	Linux     bool     `json:"linux,omitempty"`
}

type AutoPartOptions struct {
	Type             string `json:"type,omitempty"`
	FSType           string `json:"fstype,omitempty"`
	NoLVM            bool   `json:"nolvm,omitempty"`
	Encrypted        bool   `json:"encrypted,omitempty"`
	PassPhrase       string `json:"passphrase,omitempty"`
	EscrowCert       string `json:"escrowcert,omitempty"`
	BackupPassPhrase bool   `json:"backuppassphrase,omitempty"`
	Cipher           string `json:"cipher,omitempty"`
	LuksVersion      string `json:"luks-version,omitempty"`
	PBKdf            string `json:"pbkdf,omitempty"`
	PBKdfMemory      int    `json:"pbkdf-memory,omitempty"`
	PBKdfTime        int    `json:"pbkdf-time,omitempty"`
	PBKdfIterations  int    `json:"pbkdf-iterations,omitempty"`
	NoHome           bool   `json:"nohome,omitempty"`
}

type NetworkOptions struct {
	Activate    *bool    `json:"activate,omitempty"`
	BootProto   string   `json:"bootproto,omitempty"`
	Device      string   `json:"device,omitempty"`
	OnBoot      string   `json:"onboot,omitempty"`
	IP          string   `json:"ip,omitempty"`
	IPV6        string   `json:"ipv6,omitempty"`
	Gateway     string   `json:"gateway,omitempty"`
	IPV6Gateway string   `json:"ipv6gateway,omitempty"`
	Nameservers []string `json:"nameservers,omitempty"`
	Netmask     string   `json:"netmask,omitempty"`
	Hostname    string   `json:"hostname,omitempty"`
	ESSid       string   `json:"essid,omitempty"`
	WPAKey      string   `json:"wpakey,omitempty"`
}

type RootPasswordOptions struct {
	Lock      bool   `json:"lock,omitempty"`
	PlainText bool   `json:"plaintext,omitempty"`
	IsCrypted bool   `json:"iscrypted,omitempty"`
	AllowSSH  bool   `json:"allow_ssh,omitempty"`
	Password  string `json:"password,omitempty"`
}

func (KickstartStageOptions) isStageOptions() {}

// Creates an Anaconda kickstart file
func NewKickstartStage(options *KickstartStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.kickstart",
		Options: options,
	}
}

func NewKickstartStageOptions(
	path string,
	userCustomizations []users.User,
	groupCustomizations []users.Group) (*KickstartStageOptions, error) {

	var users map[string]UsersStageOptionsUser
	if usersOptions, err := NewUsersStageOptions(userCustomizations, false); err != nil {
		return nil, err
	} else if usersOptions != nil {
		users = usersOptions.Users
	}

	var groups map[string]GroupsStageOptionsGroup
	if groupsOptions := NewGroupsStageOptions(groupCustomizations); groupsOptions != nil {
		groups = groupsOptions.Groups
	}

	return &KickstartStageOptions{
		Path:         path,
		OSTreeCommit: nil,
		LiveIMG:      nil,
		Users:        users,
		Groups:       groups,
	}, nil
}

func NewKickstartStageOptionsWithOSTreeCommit(
	path string,
	userCustomizations []users.User,
	groupCustomizations []users.Group,
	ostreeURL string,
	ostreeRef string,
	ostreeRemote string,
	osName string) (*KickstartStageOptions, error) {

	options, err := NewKickstartStageOptions(path, userCustomizations, groupCustomizations)

	if err != nil {
		return nil, err
	}

	if ostreeURL != "" {
		ostreeCommitOptions := &OSTreeCommitOptions{
			OSName: osName,
			Remote: ostreeRemote,
			URL:    ostreeURL,
			Ref:    ostreeRef,
			GPG:    false,
		}

		options.OSTreeCommit = ostreeCommitOptions
	}

	return options, nil
}

func NewKickstartStageOptionsWithOSTreeContainer(
	path string,
	userCustomizations []users.User,
	groupCustomizations []users.Group,
	containerURL string,
	containerTransport string,
	containerRemote string,
	containerStateRoot string) (*KickstartStageOptions, error) {

	options, err := NewKickstartStageOptions(path, userCustomizations, groupCustomizations)

	if err != nil {
		return nil, err
	}

	if containerURL != "" {
		ostreeContainerOptions := &OSTreeContainerOptions{
			StateRoot:             containerStateRoot,
			URL:                   containerURL,
			Remote:                containerRemote,
			Transport:             containerTransport,
			SignatureVerification: false,
		}

		options.OSTreeContainer = ostreeContainerOptions
	}

	return options, nil
}

func NewKickstartStageOptionsWithLiveIMG(
	path string,
	userCustomizations []users.User,
	groupCustomizations []users.Group,
	imageURL string) (*KickstartStageOptions, error) {

	options, err := NewKickstartStageOptions(path, userCustomizations, groupCustomizations)

	if err != nil {
		return nil, err
	}

	if imageURL != "" {
		liveImg := &LiveIMGOptions{
			URL: imageURL,
		}
		options.LiveIMG = liveImg
	}

	return options, nil
}

// IncludeRaw is used for adding raw text as an extension to the kickstart
// file. First it changes the filename of the existing kickstart stage options
// and then creates a new file with the given raw content and an %include
// statement at the top that points to the renamed file. The new raw content is
// generated in place of the original file and is returned as an fsnode.File.
// The raw content *should not* contain the %include statement.
func (options *KickstartStageOptions) IncludeRaw(raw string) (*fsnode.File, error) {
	origPath := options.Path
	origName := filepath.Base(origPath)

	ext := filepath.Ext(origName)

	// file.ext -> file-base.ext
	newBaseName := strings.TrimSuffix(origName, ext) + "-base" + ext
	options.Path = filepath.Join("/", newBaseName)

	// include must point to full path when booted
	includePath := filepath.Join("/run/install/repo", newBaseName)

	rawBits := fmt.Sprintf("%%include %s\n%s", includePath, raw)
	return fsnode.NewFile(origPath, nil, nil, nil, []byte(rawBits))
}
