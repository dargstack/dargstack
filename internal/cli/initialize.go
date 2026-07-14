package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/giturl"
	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/prompt"
)

var configOnly bool
var initTarget string

var initCmd = &cobra.Command{
	Use:     "initialize [name]",
	Aliases: []string{"init"},
	Short:   "Bootstrap a new dargstack project",
	Long: `Bootstrap a new dargstack project.

Creates a project directory structure with:
- ` + "`dargstack.yaml`" + ` config file with all options (commented with defaults)
- ` + "`src/development`" + ` and ` + "`src/production`" + ` service directories
- ` + "`artifacts`" + ` directory for generated outputs (docs, certificates, audit logs)

Without arguments, prompts for a project name.
With an argument, uses it as the project name directly.

Use ` + "`--configuration-only`" + ` to print a full config template to stdout without creating a project.
Use ` + "`--target`" + ` to specify the parent directory for the project (default: current directory).

DEPRECATED: passing a Git URL to ` + "`init`" + ` will clone the repository.
Use ` + "`dargstack clone`" + ` instead.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&configOnly, "configuration-only", false, "print config template to stdout without creating a project")
	initCmd.Flags().StringVar(&initTarget, "target", "", "parent directory for the project (default: current directory)")
}

func runInit(cmd *cobra.Command, args []string) error {
	if configOnly {
		fmt.Print(generateConfigTemplate("example"))
		return nil
	}

	var name string
	if len(args) > 0 {
		name = args[0]
	}

	if name == "" {
		if noInteraction {
			return fmt.Errorf("--no-interaction requires a name argument")
		}

		var err error
		name, err = prompt.Input("Project name", "my-project")
		if err != nil {
			return err
		}
	}

	if name == "" {
		return fmt.Errorf("project name is required")
	}

	isClone := isGitURL(name)
	if isClone {
		logger.L.Warn("Passing a Git URL to `init` is deprecated; use `dargstack clone` instead.")
	}

	target := initTarget
	targetExplicit := cmd.Flags().Changed("target")
	if !targetExplicit {
		if isClone {
			target = giturl.RepoNameFromURL(name)
		} else {
			target = "."
		}
	}

	if !targetExplicit && !noInteraction {
		promptTitle := "Clone into directory"
		if !isClone {
			promptTitle = "Create project in directory"
		}
		result, err := prompt.Input(promptTitle, target)
		if err != nil {
			return err
		}
		target = result
	}

	if target == "" {
		return fmt.Errorf("target directory is required")
	}

	if isClone {
		return cloneProject(name, target)
	}
	return bootstrapProject(name, target)
}

func isGitURL(s string) bool {
	return strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "git@") ||
		strings.HasPrefix(s, "git://") ||
		strings.HasPrefix(s, "ssh://")
}

func cloneProject(url, target string) error {
	displayTarget := target
	if !filepath.IsAbs(target) {
		displayTarget = "./" + target
	}

	logger.L.Info(fmt.Sprintf("Cloning %s into %s ...", url, displayTarget))
	gitCmd := exec.Command("git", "clone", url, target) // #nosec G204 — URL is user-supplied intentionally
	gitCmd.Stdout = os.Stdout
	gitCmd.Stderr = os.Stderr
	if err := gitCmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	logger.Success(fmt.Sprintf("Project cloned into %s", displayTarget))
	logger.L.Info("Run `cd` into the directory and then `dargstack deploy` to start.")
	return nil
}

func bootstrapProject(name, target string) error {
	projectDir := filepath.Join(target, name)
	stackDir := filepath.Join(projectDir, "stack")
	helloDir := filepath.Join(projectDir, "hello")

	if _, err := os.Stat(projectDir); err == nil {
		return hintErr(
			fmt.Errorf("directory %q already exists", projectDir),
			"Choose a different name, or remove the existing directory first.",
		)
	}

	dirs := []string{
		filepath.Join(stackDir, "src", "development", "hello"),
		filepath.Join(stackDir, "src", "production", "hello"),
		filepath.Join(stackDir, "artifacts", "docs"),
		filepath.Join(stackDir, "artifacts", "audit-log"),
		filepath.Join(stackDir, "artifacts", "certificates"),
		filepath.Join(stackDir, "artifacts", "secrets"),
		helloDir,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
	}

	configContent := generateConfigTemplate(name)
	if err := os.WriteFile(filepath.Join(stackDir, "dargstack.yaml"), []byte(configContent), 0o644); err != nil {
		return fmt.Errorf("write dargstack.yaml: %w", err)
	}

	artifactsGitignore := "audit-log/\ncertificates/\nsecrets/\n"
	if err := os.WriteFile(filepath.Join(stackDir, "artifacts", ".gitignore"), []byte(artifactsGitignore), 0o644); err != nil {
		return fmt.Errorf("write artifacts/.gitignore: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stackDir, "artifacts", "README.md"), []byte(initReadmeArtifacts), 0o644); err != nil {
		return fmt.Errorf("write artifacts/README.md: %w", err)
	}

	for _, envFile := range []string{
		filepath.Join(stackDir, "src", "development", ".env"),
		filepath.Join(stackDir, "src", "production", ".env"),
	} {
		if err := os.WriteFile(envFile, []byte("# Add environment variables here (KEY=VALUE)\n"), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", envFile, err)
		}
	}

	readmes := map[string]string{
		filepath.Join(stackDir, "src", "development"): initReadmeDev,
		filepath.Join(stackDir, "src", "production"):  initReadmeProd,
	}
	for dir, content := range readmes {
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(content), 0o644); err != nil {
			return fmt.Errorf("write README.md in %s: %w", dir, err)
		}
	}

	// Example service: hello compose definition inside the stack
	helloCompose := fmt.Sprintf(initHelloCompose, name)
	if err := os.WriteFile(filepath.Join(stackDir, "src", "development", "hello", "compose.yaml"), []byte(helloCompose), 0o644); err != nil {
		return fmt.Errorf("write hello compose.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stackDir, "src", "development", "hello", "config.yaml"), []byte(initHelloDevConfig), 0o644); err != nil {
		return fmt.Errorf("write hello config.yaml: %w", err)
	}

	helloProdCompose := fmt.Sprintf(initHelloProdCompose, name)
	if err := os.WriteFile(filepath.Join(stackDir, "src", "production", "hello", "compose.yaml"), []byte(helloProdCompose), 0o644); err != nil {
		return fmt.Errorf("write production hello compose.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stackDir, "src", "production", "hello", "config.yaml"), []byte(initHelloProdConfig), 0o644); err != nil {
		return fmt.Errorf("write production hello config.yaml: %w", err)
	}

	// Example service: hello source code next to the stack directory
	if err := os.WriteFile(filepath.Join(helloDir, "Dockerfile"), []byte(initHelloDockerfile), 0o644); err != nil {
		return fmt.Errorf("write hello Dockerfile: %w", err)
	}
	if err := os.WriteFile(filepath.Join(helloDir, "main.go"), []byte(initHelloMain), 0o644); err != nil {
		return fmt.Errorf("write hello main.go: %w", err)
	}

	// Write project-level README as a sibling to the stack directory
	projectReadme := fmt.Sprintf(initReadmeProject, name, name)
	if err := os.WriteFile(filepath.Join(projectDir, "README.md"), []byte(projectReadme), 0o644); err != nil {
		return fmt.Errorf("write project README.md: %w", err)
	}

	logger.Success(fmt.Sprintf("Project %q bootstrapped at %s", name, stackDir))
	logger.L.Info(fmt.Sprintf("Next steps:\n  cd %s\n  dargstack deploy", stackDir))
	return nil
}

const initReadmeDev = `# Development Services

Create one directory per service here, each containing a ` + "`compose.yaml`" + `.

All resources belonging to a service (secrets, config files, etc.) live alongside the compose file in the service directory.

**Example** ` + "`nginx/compose.yaml`" + `:

` + "```yaml" + `
services:
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
    secrets:
      - nginx-password

secrets:
  nginx-password:
    file: ./password.secret

x-dargstack:
  secrets:
    nginx-password:
      length: 32
      special_characters: false
` + "```" + `

## Profiles

Services without a ` + "`deploy.labels.dargstack.profiles`" + ` label are **always deployed**.
To make a service opt-in, add:

` + "```yaml" + `
deploy:
  labels:
    dargstack.profiles: myprofile
` + "```" + `

Then deploy with ` + "`dargstack deploy --profiles myprofile`" + `.

If no profile is given and any service declares a ` + "`default`" + ` profile, only the default group is deployed.
`

const initReadmeProd = `# Production Service Overrides

Create one directory for each service here, that requires a production override.

- ` + "`compose.yaml`" + ` files are deep-merged on top of the corresponding development service files
  - see [github.com: What are all the Spruce operators?](https://github.com/geofffranks/spruce/blob/main/doc/operators.md) for special keywords controlling the merge behavior
- configuration files replace their development counterparts
- secrets turn ` + "`external`" + ` automatically, so they don't need to be overridden with files here
- environment variables extend the development variables, potentially overriding value if keys match

**Example** ` + "`nginx/compose.yaml`" + ` that pins the image to a release tag:

` + "```yaml" + `
services:
  nginx:
    image: nginx:1.27
` + "```" + `

## Dev-only lines

Any line in a development service file annotated with ` + "`# dargstack:dev-only`" + ` is stripped before the production merge.
`

const initReadmeProject = `# %s

This directory is the project root. The ` + "`stack/`" + ` subdirectory contains the
dargstack configuration and service definitions.
Service source code lives as siblings to ` + "`stack/`" + `.

` + "```" + `
%s/
├── stack/                         # dargstack project (compose files, config, secrets)
│   └── src/
│       ├── development/
│       │   └── hello/             # development service definition
│       │       ├── compose.yaml   # service spec (build label, port, config, secret)
│       │       └── config.yaml    # development config mounted into the container
│       └── production/
│           └── hello/             # production overrides
│               ├── compose.yaml   # Spruce operators: purge ports, append labels, external secret
│               └── config.yaml    # production config replaces the development one
├── hello/                         # example service source (build this with Docker)
│   ├── Dockerfile
│   └── main.go
└── README.md                      # this file
` + "```" + `

## Getting started

` + "```bash" + `
cd stack
dargstack deploy
` + "```" + `

The ` + "`hello`" + ` service is built automatically from ` + "`hello/`" + ` and served on port 8080.
Replace it with your own services — clone source repositories next to ` + "`stack/`" + ` and
add service directories in ` + "`stack/src/development/`" + ` and ` + "`stack/src/production/`" + `.
`

const initReadmeArtifacts = `# Artifacts

This directory contains generated outputs and runtime artifacts produced by dargstack.

## Contents

- ` + "`audit-log/`" + `: deployment snapshots for audit/history.
- ` + "`certificates/`" + `: local development TLS certificates.
- ` + "`docs/`" + `: generated stack documentation (` + "`README.md`" + `).
- ` + "`secrets/`" + `: Docker secrets managed by dargstack. Secret files are created here during ` + "`dargstack deploy`" + ` and referenced by compose definitions. Secrets can be auto-generated from ` + "`x-dargstack.secrets`" + ` metadata or set interactively.

## Version Control

` + "`audit-log/`" + `, ` + "`certificates/`" + `, and ` + "`secrets/`" + ` are ignored via ` + "`artifacts/.gitignore`" + `.
` + "`docs/`" + ` is tracked so generated documentation can be shared.
`

const initHelloDockerfile = `FROM golang:alpine AS builder
WORKDIR /app
COPY main.go .
RUN go mod init hello && go build -o hello .

FROM alpine
COPY --from=builder /app/hello /usr/local/bin/hello
EXPOSE 8080
CMD ["hello"]
`

const initHelloMain = `package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Read greeting from the mounted config file.
		greeting := "Hello from dargstack!"
		if data, err := os.ReadFile("/etc/hello/config.yaml"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "greeting: ") {
					greeting = strings.TrimPrefix(line, "greeting: ")
				}
			}
		}
		fmt.Fprintln(w, greeting)
	})
	_ = http.ListenAndServe(":8080", nil)
}
`

// initHelloCompose is the development service definition for the example hello service.
// The dargstack.development.build label points to the hello source directory that lives
// next to the stack/ directory (../../../../hello from this service's directory).
const initHelloCompose = `configs:
  hello-config:
    file: ./config.yaml

secrets:
  hello-api-key:
    file: ./api-key.secret

services:
  hello:
    configs:
      - source: hello-config
        target: /etc/hello/config.yaml
    deploy:
      labels:
        # Build context relative to stack/src/development/hello/ — points to <project>/hello/
        dargstack.development.build: "../../../../hello"
        # Alternatively, clone from a git repo (clones to sibling of stack directory):
        # dargstack.development.git: "git@github.com:yourorg/hello.git"
    image: %s/hello:development
    ports:
      - "8080:8080"
    secrets:
      - hello-api-key

x-dargstack:
  secrets:
    hello-api-key:
      length: 32
      special_characters: false
`

const initHelloDevConfig = `# Development configuration for the hello service.
debug: true
greeting: Hello from dargstack!
`

// initHelloProdCompose is the production overlay for the example hello service.
// It demonstrates the three most useful Spruce merge operators:
//   - plain key overwrite (image tag)
//   - (( purge )) to remove a development-only key
//   - (( append )) to extend a list without replacing it
const initHelloProdCompose = `# Production overrides for the hello service.
# Spruce operators used here:
#   (( purge ))  — remove this key from the merged result
#   (( append )) — append to the list instead of replacing it
# All other keys simply overwrite the development value.

configs:
  hello-config:
    file: ./config.yaml      # overwrite: use the production config file

secrets:
  # Secrets with file: in development are automatically converted to external: true
  # in production. No manual override needed.

services:
  hello:
    image: %s/hello:latest   # overwrite: pin to a versioned release tag
    ports: (( purge ))       # purge: no direct port binding in production (use an ingress)
    deploy:
      labels:
        - (( append ))       # append: keep existing dev labels and add new ones
        - "traefik.enable=true"
`

const initHelloProdConfig = `# Production configuration for the hello service.
debug: false
greeting: Hello from production!
`

func generateConfigTemplate(name string) string {
	return fmt.Sprintf(`# Dargstack configuration file

metadata:
  name: %q # optional, defaults to parent directory name
  compatibility: ">=4.0.0 <5.0.0" # required, CLI version constraint

  # source:
  #   name: %q
  #   url: "https://github.com/example/%s"

runtime:
  sudo: auto # optional, "auto" | "always" | "never"
  build:
    mode: always # optional, "always" | "missing"
  deploy:
    volumes:
      prompt: true # optional, defaults to true

environment:
  development:
    domain: app.localhost # optional, defaults to "app.localhost"
    certificate:
      include: [] # optional, domains added to TLS cert
        # - "*.app.localhost"
      exclude: [] # optional, domains removed from TLS cert
        # - "admin.app.localhost"

  production:
    domain: app.localhost # optional, defaults to "app.localhost"
    branch: main # optional, defaults to "main"
    # tag: 1.0.0 # optional, defaults to auto-detection
`, name, name, name)
}
