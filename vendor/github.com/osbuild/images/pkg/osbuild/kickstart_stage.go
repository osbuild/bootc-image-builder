package osbuild

import "github.com/osbuild/images/internal/users"

type KickstartStageOptions struct {
	// Where to place the kickstart file
	Path string `json:"path"`

	OSTree *OSTreeOptions `json:"ostree,omitempty"`

	LiveIMG *LiveIMG `json:"liveimg,omitempty"`

	Users map[string]UsersStageOptionsUser `json:"users,omitempty"`

	Groups map[string]GroupsStageOptionsGroup `json:"groups,omitempty"`
}

type LiveIMG struct {
	URL string `json:"url"`
}

type OSTreeOptions struct {
	OSName string `json:"osname"`
	URL    string `json:"url"`
	Ref    string `json:"ref"`
	GPG    bool   `json:"gpg"`
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
	imageURL string,
	userCustomizations []users.User,
	groupCustomizations []users.Group,
	ostreeURL string,
	ostreeRef string,
	osName string) (*KickstartStageOptions, error) {

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

	var ostreeOptions *OSTreeOptions
	if ostreeURL != "" {
		ostreeOptions = &OSTreeOptions{
			OSName: osName,
			URL:    ostreeURL,
			Ref:    ostreeRef,
			GPG:    false,
		}
	}

	var liveImg *LiveIMG
	if imageURL != "" {
		liveImg = &LiveIMG{
			URL: imageURL,
		}
	}
	return &KickstartStageOptions{
		Path:    path,
		OSTree:  ostreeOptions,
		LiveIMG: liveImg,
		Users:   users,
		Groups:  groups,
	}, nil
}
