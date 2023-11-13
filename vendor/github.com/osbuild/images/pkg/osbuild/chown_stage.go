package osbuild

import (
	"fmt"
	"regexp"
)

const (
	// should be "^\\/(?!\\.\\.)((?!\\/\\.\\.\\/).)+$" but Go doesn't support lookaheads
	// therefore we have to instead check for the invalid cases, which is much simpler
	chownStageInvalidPathRegex = `((^|\/)[.]{2}(\/|$))|^([^/].*)*$`
	chownStageUsernameRegex    = `^[A-Za-z0-9_.][A-Za-z0-9_.-]{0,31}$`
	chownStageGroupnameRegex   = `^[A-Za-z0-9_][A-Za-z0-9_-]{0,31}$`
)

type ChownStageOptions struct {
	Items map[string]ChownStagePathOptions `json:"items"`
}

func (ChownStageOptions) isStageOptions() {}

func (o *ChownStageOptions) validate() error {
	for path, options := range o.Items {
		invalidPathRegex := regexp.MustCompile(chownStageInvalidPathRegex)
		if invalidPathRegex.FindAllString(path, -1) != nil {
			return fmt.Errorf("chown path %q matches invalid path pattern (%s)", path, invalidPathRegex.String())
		}

		if err := options.validate(); err != nil {
			return err
		}
	}

	return nil
}

type ChownStagePathOptions struct {
	// User can be either a string (user name), an int64 (UID) or nil
	User interface{} `json:"user,omitempty"`
	// Group can be either a string (grou pname), an int64 (GID) or nil
	Group interface{} `json:"group,omitempty"`

	Recursive bool `json:"recursive,omitempty"`
}

// validate checks that the options values conform to the schema
func (o *ChownStagePathOptions) validate() error {
	switch user := o.User.(type) {
	case string:
		usernameRegex := regexp.MustCompile(chownStageUsernameRegex)
		if !usernameRegex.MatchString(user) {
			return fmt.Errorf("chown user name %q doesn't conform to schema (%s)", user, usernameRegex.String())
		}
	case int64:
		if user < 0 {
			return fmt.Errorf("chown user id %d is negative", user)
		}
	case nil:
		// user is not set
	default:
		return fmt.Errorf("chown user must be either a string nor an int64, got %T", user)
	}

	switch group := o.Group.(type) {
	case string:
		groupnameRegex := regexp.MustCompile(chownStageGroupnameRegex)
		if !groupnameRegex.MatchString(group) {
			return fmt.Errorf("chown group name %q doesn't conform to schema (%s)", group, groupnameRegex.String())
		}
	case int64:
		if group < 0 {
			return fmt.Errorf("chown group id %d is negative", group)
		}
	case nil:
		// group is not set
	default:
		return fmt.Errorf("chown group must be either a string nor an int64, got %T", group)
	}

	// check that at least one of user or group is set
	if o.User == nil && o.Group == nil {
		return fmt.Errorf("chown user and group are both not set")
	}

	return nil
}

// NewChownStage creates a new org.osbuild.chown stage
func NewChownStage(options *ChownStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.chown",
		Options: options,
	}
}
