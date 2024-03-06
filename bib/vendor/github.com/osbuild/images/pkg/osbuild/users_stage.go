package osbuild

import (
	"github.com/osbuild/images/pkg/crypt"
	"github.com/osbuild/images/pkg/customizations/users"
)

type UsersStageOptions struct {
	Users map[string]UsersStageOptionsUser `json:"users"`
}

func (UsersStageOptions) isStageOptions() {}

type UsersStageOptionsUser struct {
	UID         *int     `json:"uid,omitempty"`
	GID         *int     `json:"gid,omitempty"`
	Groups      []string `json:"groups,omitempty"`
	Description *string  `json:"description,omitempty"`
	Home        *string  `json:"home,omitempty"`
	Shell       *string  `json:"shell,omitempty"`
	Password    *string  `json:"password,omitempty"`
	Key         *string  `json:"key,omitempty"`
}

func NewUsersStage(options *UsersStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.users",
		Options: options,
	}
}

func NewUsersStageOptions(userCustomizations []users.User, omitKey bool) (*UsersStageOptions, error) {
	if len(userCustomizations) == 0 {
		return nil, nil
	}

	users := make(map[string]UsersStageOptionsUser, len(userCustomizations))
	for _, uc := range userCustomizations {
		// Don't hash empty passwords, set to nil to lock account
		if uc.Password != nil && len(*uc.Password) == 0 {
			uc.Password = nil
		}

		// Hash non-empty un-hashed passwords
		if uc.Password != nil && !crypt.PasswordIsCrypted(*uc.Password) {
			cryptedPassword, err := crypt.CryptSHA512(*uc.Password)
			if err != nil {
				return nil, err
			}

			uc.Password = &cryptedPassword
		}

		user := UsersStageOptionsUser{
			UID:         uc.UID,
			GID:         uc.GID,
			Groups:      uc.Groups,
			Description: uc.Description,
			Home:        uc.Home,
			Shell:       uc.Shell,
			Password:    uc.Password,
			Key:         nil,
		}
		if !omitKey {
			user.Key = uc.Key
		}
		users[uc.Name] = user
	}

	return &UsersStageOptions{Users: users}, nil
}

func GenUsersStage(users []users.User, omitKey bool) (*Stage, error) {
	options, err := NewUsersStageOptions(users, omitKey)
	if err != nil {
		return nil, err
	}
	return NewUsersStage(options), nil
}
