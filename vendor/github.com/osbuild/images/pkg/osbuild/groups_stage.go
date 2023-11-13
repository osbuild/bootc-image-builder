package osbuild

import (
	"github.com/osbuild/images/internal/users"
)

type GroupsStageOptions struct {
	Groups map[string]GroupsStageOptionsGroup `json:"groups"`
}

func (GroupsStageOptions) isStageOptions() {}

type GroupsStageOptionsGroup struct {
	GID *int `json:"gid,omitempty"`
}

func NewGroupsStage(options *GroupsStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.groups",
		Options: options,
	}
}

func NewGroupsStageOptions(groups []users.Group) *GroupsStageOptions {
	options := GroupsStageOptions{
		Groups: map[string]GroupsStageOptionsGroup{},
	}

	for _, group := range groups {
		options.Groups[group.Name] = GroupsStageOptionsGroup{
			GID: group.GID,
		}
	}

	return &options
}

func GenGroupsStage(groups []users.Group) *Stage {
	options := &GroupsStageOptions{
		Groups: make(map[string]GroupsStageOptionsGroup, len(groups)),
	}
	for _, group := range groups {
		options.Groups[group.Name] = GroupsStageOptionsGroup{
			GID: group.GID,
		}
	}
	return NewGroupsStage(options)
}
