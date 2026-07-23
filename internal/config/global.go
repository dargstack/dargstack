package config

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const (
	GlobalConfigDir  = ".config/dargstack"
	GlobalConfigFile = "config.yaml"
)

// GlobalConfig mirrors the project Config structure for user-level settings.
// Only a subset of fields is supported; unknown fields are ignored.
type GlobalConfig struct {
	Runtime struct {
		Skill SkillConfig `yaml:"skill"`
	} `yaml:"runtime"`
}

// GlobalConfigPath returns the path to the global config file.
// Returns an empty string if the home directory is unavailable.
func GlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, GlobalConfigDir, GlobalConfigFile)
}

// LoadGlobalConfig reads and parses the global config file.
// Returns a GlobalConfig with defaults applied. Missing file is not an error.
func LoadGlobalConfig() (*GlobalConfig, error) {
	cfg := &GlobalConfig{}
	cfg.applyDefaults()

	path := GlobalConfigPath()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read global config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse global config %s: %w", path, err)
	}
	cfg.applyDefaults()

	return cfg, nil
}

func (c *GlobalConfig) applyDefaults() {
	if c.Runtime.Skill.Install == "" {
		c.Runtime.Skill.Install = SkillInstallAuto
	}
}

// EffectiveSkillInstall returns the effective skill install mode.
// Project config overrides global config. If the project config has a
// non-default value, it takes precedence. Otherwise, global config is used.
func EffectiveSkillInstall(global *GlobalConfig, project *Config) SkillInstallMode {
	// If project config explicitly set a non-default value, use it.
	if project.Runtime.Skill.Install != SkillInstallAuto {
		return project.Runtime.Skill.Install
	}
	return global.Runtime.Skill.Install
}
