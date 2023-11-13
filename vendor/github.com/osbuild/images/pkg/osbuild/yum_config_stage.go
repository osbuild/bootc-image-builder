package osbuild

import (
	"fmt"
)

type YumConfigConfig struct {
	HttpCaching *string `json:"http_caching,omitempty"`
}

type YumConfigPlugins struct {
	Langpacks *YumConfigPluginsLangpacks `json:"langpacks,omitempty"`
}

type YumConfigPluginsLangpacks struct {
	Locales []string `json:"locales"`
}

type YumConfigStageOptions struct {
	Config  *YumConfigConfig  `json:"config,omitempty"`
	Plugins *YumConfigPlugins `json:"plugins,omitempty"`
}

func (YumConfigStageOptions) isStageOptions() {}

func (o YumConfigStageOptions) validate() error {
	// Allow values from the osbuild schema
	if o.Config != nil && o.Config.HttpCaching != nil {
		valid := false
		allowed_http_caching_values := []string{"all", "packages", "lazy:packages", "none"}
		for _, v := range allowed_http_caching_values {
			if v == *o.Config.HttpCaching {
				valid = true
			}
		}
		if !valid {
			return fmt.Errorf("yum config parameter http_caching does not allow %s as a value", *o.Config.HttpCaching)
		}
	}

	if o.Plugins != nil && o.Plugins.Langpacks != nil && len(o.Plugins.Langpacks.Locales) < 1 {
		return fmt.Errorf("locales must contain at least one element")
	}

	return nil
}

func NewYumConfigStage(options *YumConfigStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.yum.config",
		Options: options,
	}
}
