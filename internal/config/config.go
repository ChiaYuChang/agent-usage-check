package config

import (
	"os"
	"strings"
	"time"

	"agent-usage-check/internal/cache"
	"gopkg.in/yaml.v3"
)

const (
	DefaultInterval         = 2 * time.Minute
	DefaultConfigFile       = "~/.config/agent-usage-check/config.yaml"
	DefaultCacheFile        = "~/.config/agent-usage-check/cache.json"
	DefaultAuthFile         = "~/.codex/auth.json"
	DefaultAgyBin           = "agy"
	DefaultOpencodeEndpoint = "https://opencode.ai/zen/go/v1/usage"
)

type YAMLConfig struct {
	Provider            string    `yaml:"provider"`
	Display             string    `yaml:"display"`
	Interval            string    `yaml:"interval"`
	RemainingChar       string    `yaml:"remaining_char"`
	RemainingCharAlt    string    `yaml:"remaining-char"`
	UsedChar            string    `yaml:"used_char"`
	UsedCharAlt         string    `yaml:"used-char"`
	IconNode            yaml.Node `yaml:"icon"`
	CachePath           string    `yaml:"cache_path"`
	CachePathAlt        string    `yaml:"cache-path"`
	AuthPath            string    `yaml:"auth_path"`
	AuthPathAlt         string    `yaml:"auth-path"`
	AgyPath             string    `yaml:"agy_path"`
	AgyPathAlt          string    `yaml:"agy-path"`
	AgyGroup            string    `yaml:"agy_group"`
	AgyGroupAlt         string    `yaml:"agy-group"`
	OpencodeAPIKey      string    `yaml:"opencode_api_key"`
	OpencodeAPIKeyAlt   string    `yaml:"opencode-api-key"`
	OpencodeAuthPath    string    `yaml:"opencode_auth_path"`
	OpencodeAuthPathAlt string    `yaml:"opencode-auth-path"`
	OpencodeEndpoint    string    `yaml:"opencode_endpoint"`
	OpencodeEndpointAlt string    `yaml:"opencode-endpoint"`
	JSON                bool      `yaml:"json"`
	Text                bool      `yaml:"text"`
	TextFormat          string    `yaml:"text_format"`
	TextFormatAlt       string    `yaml:"text-format"`
	Watch               bool      `yaml:"watch"`
}

type Config struct {
	Provider         string
	Display          string
	Interval         time.Duration
	RemainingChar    string
	UsedChar         string
	Icon             string
	Icons            map[string]string
	CachePath        string
	AuthPath         string
	AgyPath          string
	AgyGroup         string
	OpencodeAPIKey   string
	OpencodeAuthPath string
	OpencodeEndpoint string
	JSON             bool
	Text             bool
	TextFormat       string
	Watch            bool
}

func DefaultConfig() Config {
	return Config{
		Provider:         "openai",
		Display:          "remaining",
		Interval:         DefaultInterval,
		RemainingChar:    "■",
		UsedChar:         "□",
		Icon:             "",
		Icons:            nil,
		CachePath:        DefaultCacheFile,
		AuthPath:         DefaultAuthFile,
		AgyPath:          DefaultAgyBin,
		AgyGroup:         "",
		OpencodeAPIKey:   "",
		OpencodeAuthPath: "",
		OpencodeEndpoint: DefaultOpencodeEndpoint,
		JSON:             false,
		Text:             false,
		TextFormat:       "",
		Watch:            false,
	}
}

func (c *Config) GetIcon(prov string) string {
	if c.Icons != nil {
		target := strings.ToLower(prov)
		if ic, ok := c.Icons[target]; ok {
			return ic
		}
		if target == "antigravity" {
			if ic, ok := c.Icons["agy"]; ok {
				return ic
			}
		}
	}
	return c.Icon
}

func LoadYAML(path string, cfg *Config) error {
	resolved := cache.ExpandPath(path)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return err
	}

	var yc YAMLConfig
	if err := yaml.Unmarshal(data, &yc); err != nil {
		return err
	}

	if yc.Provider != "" {
		cfg.Provider = yc.Provider
	}
	if yc.Display != "" {
		cfg.Display = yc.Display
	}
	if yc.Interval != "" {
		if d, err := time.ParseDuration(yc.Interval); err == nil {
			cfg.Interval = d
		}
	}
	if yc.RemainingChar != "" {
		cfg.RemainingChar = yc.RemainingChar
	} else if yc.RemainingCharAlt != "" {
		cfg.RemainingChar = yc.RemainingCharAlt
	}
	if yc.UsedChar != "" {
		cfg.UsedChar = yc.UsedChar
	} else if yc.UsedCharAlt != "" {
		cfg.UsedChar = yc.UsedCharAlt
	}
	if yc.IconNode.Kind == yaml.ScalarNode {
		cfg.Icon = yc.IconNode.Value
		cfg.Icons = nil
	} else if yc.IconNode.Kind == yaml.MappingNode {
		var iconMap map[string]string
		if err := yc.IconNode.Decode(&iconMap); err == nil {
			cfg.Icons = iconMap
		}
	}
	if yc.CachePath != "" {
		cfg.CachePath = yc.CachePath
	} else if yc.CachePathAlt != "" {
		cfg.CachePath = yc.CachePathAlt
	}
	if yc.AuthPath != "" {
		cfg.AuthPath = yc.AuthPath
	} else if yc.AuthPathAlt != "" {
		cfg.AuthPath = yc.AuthPathAlt
	}
	if yc.AgyPath != "" {
		cfg.AgyPath = yc.AgyPath
	} else if yc.AgyPathAlt != "" {
		cfg.AgyPath = yc.AgyPathAlt
	}
	if yc.AgyGroup != "" {
		cfg.AgyGroup = yc.AgyGroup
	} else if yc.AgyGroupAlt != "" {
		cfg.AgyGroup = yc.AgyGroupAlt
	}
	if yc.OpencodeAPIKey != "" {
		cfg.OpencodeAPIKey = yc.OpencodeAPIKey
	} else if yc.OpencodeAPIKeyAlt != "" {
		cfg.OpencodeAPIKey = yc.OpencodeAPIKeyAlt
	}
	if yc.OpencodeAuthPath != "" {
		cfg.OpencodeAuthPath = yc.OpencodeAuthPath
	} else if yc.OpencodeAuthPathAlt != "" {
		cfg.OpencodeAuthPath = yc.OpencodeAuthPathAlt
	}
	if yc.OpencodeEndpoint != "" {
		cfg.OpencodeEndpoint = yc.OpencodeEndpoint
	} else if yc.OpencodeEndpointAlt != "" {
		cfg.OpencodeEndpoint = yc.OpencodeEndpointAlt
	}
	if yc.JSON {
		cfg.JSON = yc.JSON
	}
	if yc.Text {
		cfg.Text = yc.Text
	}
	if yc.TextFormat != "" {
		cfg.TextFormat = yc.TextFormat
	} else if yc.TextFormatAlt != "" {
		cfg.TextFormat = yc.TextFormatAlt
	}
	if yc.Watch {
		cfg.Watch = yc.Watch
	}
	return nil
}
