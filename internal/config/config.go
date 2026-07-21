package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Masterminds/semver/v3"
	"go.yaml.in/yaml/v3"

	"github.com/dargstack/dargstack/v4/internal/version"
)

type SudoMode string

const (
	SudoAuto   SudoMode = "auto"
	SudoAlways SudoMode = "always"
	SudoNever  SudoMode = "never"
)

func (s *SudoMode) UnmarshalYAML(node *yaml.Node) error {
	var v string
	if err := node.Decode(&v); err != nil {
		return err
	}
	switch SudoMode(v) {
	case SudoAuto, SudoAlways, SudoNever, "":
		*s = SudoMode(v)
	default:
		return fmt.Errorf("invalid sudo mode %q: must be auto, always, or never", v)
	}
	return nil
}

type BuildMode string

const (
	BuildAlways  BuildMode = "always"
	BuildMissing BuildMode = "missing"
)

func (b *BuildMode) UnmarshalYAML(node *yaml.Node) error {
	var v string
	if err := node.Decode(&v); err != nil {
		return err
	}
	switch BuildMode(v) {
	case BuildAlways, BuildMissing, "":
		*b = BuildMode(v)
	default:
		return fmt.Errorf("invalid build mode %q: must be always or missing", v)
	}
	return nil
}

type Config struct {
	Schema   string `yaml:"$schema"` // JSON Schema URI — consumed and ignored
	stackDir string

	Environment EnvironmentConfig `yaml:"environment"`
	Metadata    MetadataConfig    `yaml:"metadata"`
	Runtime     RuntimeConfig     `yaml:"runtime"`
}

type ExternalService struct {
	Description string `yaml:"description"`
}

type MetadataConfig struct {
	Compatibility    string                     `yaml:"compatibility"`
	ExternalServices map[string]ExternalService `yaml:"external_services"`
	Name             string                     `yaml:"name"`
	Source           SourceConfig               `yaml:"source"`
}

type SourceConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type RuntimeConfig struct {
	Build  BuildConfig  `yaml:"build"`
	Deploy DeployConfig `yaml:"deploy"`
	Sudo   SudoMode     `yaml:"sudo"`
}

type BuildConfig struct {
	Mode BuildMode `yaml:"mode"`
}

type DeployConfig struct {
	Volumes DeployVolumeConfig `yaml:"volumes"`
}

type DeployVolumeConfig struct {
	Prompt *bool `yaml:"prompt"`
}

type EnvironmentConfig struct {
	Development DevConfig  `yaml:"development"`
	Production  ProdConfig `yaml:"production"`
}

type DevConfig struct {
	Certificate CertificateConfig `yaml:"certificate"`
	Domain      string            `yaml:"domain"`
}

type ProdConfig struct {
	Branch string `yaml:"branch"`
	Domain string `yaml:"domain"`
	Tag    string `yaml:"tag"`
}

type CertificateConfig struct {
	Exclude []string `yaml:"exclude"`
	Include []string `yaml:"include"`
}

const ConfigFileName = "dargstack.yaml"

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

	cfg.stackDir = stackDir
	cfg.applyDefaults()

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) applyDefaults() {
	abs, err := filepath.EvalSymlinks(c.stackDir)
	if err != nil {
		abs, err = filepath.Abs(c.stackDir)
	}
	if err == nil {
		c.stackDir = abs
	}
	if c.Metadata.Name == "" {
		c.Metadata.Name = filepath.Base(filepath.Dir(c.stackDir))
	}
	if c.Runtime.Sudo == "" {
		c.Runtime.Sudo = SudoAuto
	}
	if c.Runtime.Build.Mode == "" {
		c.Runtime.Build.Mode = BuildAlways
	}
	if c.Runtime.Deploy.Volumes.Prompt == nil {
		p := true
		c.Runtime.Deploy.Volumes.Prompt = &p
	}
	if c.Environment.Production.Branch == "" {
		c.Environment.Production.Branch = "main"
	}
	if c.Environment.Production.Domain == "" {
		c.Environment.Production.Domain = "app.localhost"
	}
	if c.Environment.Development.Domain == "" {
		c.Environment.Development.Domain = "app.localhost"
	}
}

var validStackName = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9_.-]*[a-zA-Z0-9])?$`)

func (c *Config) validate() error {
	if !validStackName.MatchString(c.Metadata.Name) {
		return fmt.Errorf("invalid stack name %q: must be alphanumeric with hyphens/underscores/dots", c.Metadata.Name)
	}
	if c.Metadata.Compatibility == "" {
		return nil
	}
	if version.Version == "dev" {
		return nil
	}

	constraint, err := semver.NewConstraint(c.Metadata.Compatibility)
	if err != nil {
		return fmt.Errorf("invalid compatibility range %q: %w", c.Metadata.Compatibility, err)
	}

	v, err := semver.NewVersion(version.Version)
	if err != nil {
		return fmt.Errorf("invalid CLI version %q: %w", version.Version, err)
	}

	if !constraint.Check(v) {
		return fmt.Errorf(
			"cli version %s does not satisfy project compatibility range %s — please update dargstack",
			version.Version, c.Metadata.Compatibility,
		)
	}
	return nil
}

func (c *Config) ArtifactsDir() string { return filepath.Join(c.stackDir, "artifacts") }
func (c *Config) CertificatesDir() string {
	return filepath.Join(c.stackDir, "artifacts", "certificates")
}
func (c *Config) DevDir() string      { return filepath.Join(c.stackDir, "src", "development") }
func (c *Config) DevEnvFile() string  { return filepath.Join(c.stackDir, "src", "development", ".env") }
func (c *Config) ProdDir() string     { return filepath.Join(c.stackDir, "src", "production") }
func (c *Config) ProdEnvFile() string { return filepath.Join(c.stackDir, "src", "production", ".env") }
func (c *Config) SecretsDir() string  { return filepath.Join(c.stackDir, "artifacts", "secrets") }
func (c *Config) StackDir() string    { return c.stackDir }

// CollectServiceFiles returns the paths to compose.yaml files in the given
// directory. It includes the shared compose.yaml at the directory root if it
// exists, plus compose.yaml from each subdirectory that contains one.
func CollectServiceFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}

	var files []string

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
