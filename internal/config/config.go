package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"

	"github.com/dargstack/dargstack/v4/internal/version"
)

type Config struct {
	Compatibility string            `yaml:"compatibility"`
	Name          string            `yaml:"name"`
	Sudo          string            `yaml:"sudo"`
	Behavior      BehaviorConfig    `yaml:"behavior"`
	Production    ProductionConfig  `yaml:"production"`
	Development   DevelopmentConfig `yaml:"development"`
	Source        SourceConfig      `yaml:"source"`
}

type BehaviorConfig struct {
	Build  *BuildBehavior  `yaml:"build"`
	Prompt *PromptBehavior `yaml:"prompt"`
}

type BuildBehavior struct {
	Skip bool `yaml:"skip"`
}

type PromptBehavior struct {
	Volume *VolumeBehavior `yaml:"volume"`
}

type VolumeBehavior struct {
	Remove bool `yaml:"remove"`
}

type ProductionConfig struct {
	Branch string `yaml:"branch"`
	Tag    string `yaml:"tag"`
	Domain string `yaml:"domain"`
}

type DevelopmentConfig struct {
	Domains []string `yaml:"domains"`
}

type SourceConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

const ConfigFileName = "dargstack.yaml"

// DetectStackDir walks up from the current directory to find dargstack.yaml.
func DetectStackDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		candidate := filepath.Join(dir, ConfigFileName)
		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%s not found in any parent directory", ConfigFileName)
		}
		dir = parent
	}
}

// Load reads and parses dargstack.yaml from the given directory.
func Load(stackDir string) (*Config, error) {
	path := filepath.Join(stackDir, ConfigFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	cfg.applyDefaults(stackDir)

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) applyDefaults(stackDir string) {
	if c.Name == "" {
		// Normalize to an absolute path so relative inputs like --config stack
		// don't produce "." as the derived project name.
		abs, err := filepath.Abs(stackDir)
		if err == nil {
			stackDir = abs
		}
		c.Name = filepath.Base(filepath.Dir(stackDir))
	}
	if c.Sudo == "" {
		c.Sudo = "auto"
	}
	if c.Production.Branch == "" {
		c.Production.Branch = "main"
	}
	if c.Production.Tag == "" {
		c.Production.Tag = "latest"
	}
	if c.Production.Domain == "" {
		c.Production.Domain = "app.localhost"
	}
}

var validStackName = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9_.-]*[a-zA-Z0-9])?$`)

func (c *Config) validate() error {
	if !validStackName.MatchString(c.Name) {
		return fmt.Errorf("invalid stack name %q: must be alphanumeric with hyphens/underscores/dots", c.Name)
	}
	return nil
}

// CheckCompatibility verifies the CLI version satisfies the project's compatibility range.
func (c *Config) CheckCompatibility() error {
	if c.Compatibility == "" {
		return nil
	}
	if version.Version == "dev" {
		return nil
	}

	constraint, err := semver.NewConstraint(c.Compatibility)
	if err != nil {
		return fmt.Errorf("invalid compatibility range %q: %w", c.Compatibility, err)
	}

	v, err := semver.NewVersion(version.Version)
	if err != nil {
		return fmt.Errorf("invalid CLI version %q: %w", version.Version, err)
	}

	if !constraint.Check(v) {
		return fmt.Errorf(
			"CLI version %s does not satisfy project compatibility range %s — please update dargstack",
			version.Version, c.Compatibility,
		)
	}
	return nil
}

// DevDir returns the path to the development source directory.
func DevDir(stackDir string) string {
	return filepath.Join(stackDir, "src", "development")
}

// ProdDir returns the path to the production source directory.
func ProdDir(stackDir string) string {
	return filepath.Join(stackDir, "src", "production")
}

// ArtifactsDir returns the path to the artifacts directory.
func ArtifactsDir(stackDir string) string {
	return filepath.Join(stackDir, "artifacts")
}

// CertificatesDir returns the path to the TLS certificates directory.
func CertificatesDir(stackDir string) string {
	return filepath.Join(stackDir, "artifacts", "certificates")
}

// SecretsDir returns the path to the generated secrets directory.
func SecretsDir(stackDir string) string {
	return filepath.Join(stackDir, "artifacts", "secrets")
}

// DevEnvFile returns the path to the development .env file.
func DevEnvFile(stackDir string) string {
	return filepath.Join(stackDir, "src", "development", ".env")
}

// ProdEnvFile returns the path to the production .env file.
func ProdEnvFile(stackDir string) string {
	return filepath.Join(stackDir, "src", "production", ".env")
}

// CollectServiceFiles returns compose.yaml paths from service directories.
// If a shared compose.yaml exists directly in dir, it is included first as the base layer.
// Each subdirectory of dir that contains a compose.yaml is treated as a service.
func CollectServiceFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}

	var files []string

	// Shared compose.yaml at the environment root (base layer).
	sharedPath := filepath.Join(dir, "compose.yaml")
	if _, err := os.Stat(sharedPath); err == nil {
		files = append(files, sharedPath)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		composePath := filepath.Join(dir, e.Name(), "compose.yaml")
		if _, err := os.Stat(composePath); err == nil {
			files = append(files, composePath)
		}
	}
	return files, nil
}
