package compose

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

// EnsureEnvFile copies templatePath to envPath if envPath does not exist.
// If templatePath does not exist, it creates envPath with a minimal placeholder comment.
func EnsureEnvFile(envPath, templatePath string) error {
	_, err := os.Stat(envPath)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("stat env file %s: %w", envPath, err)
	}

	data, err := os.ReadFile(templatePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read env template %s: %w", templatePath, err)
	}

	if os.IsNotExist(err) {
		data = []byte("# Add environment variables here (KEY=VALUE)\n")
	}

	return os.WriteFile(envPath, data, 0o644)
}

// MergeEnvFiles merges .env files from development and production.
// Production values override development values.
// Returns merged content.
func MergeEnvFiles(devEnv, prodEnv string) ([]byte, error) {
	env := make(map[string]string)

	if err := loadEnvFile(devEnv, env); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read dev env: %w", err)
	}

	if err := loadEnvFile(prodEnv, env); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read prod env: %w", err)
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, env[k])
	}

	return []byte(b.String()), nil
}

func loadEnvFile(path string, env map[string]string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.IndexByte(line, '='); idx > 0 {
			env[line[:idx]] = line[idx+1:]
		}
	}
	return scanner.Err()
}

// LoadEnvFile reads a .env file and returns key-value pairs.
func LoadEnvFile(path string) (map[string]string, error) {
	env := make(map[string]string)
	err := loadEnvFile(path, env)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return env, nil
}

// FindMissingEnvValues returns keys that have empty values in the env map.
func FindMissingEnvValues(env map[string]string) []string {
	var missing []string
	for k, v := range env {
		if v == "" {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	return missing
}

// WriteEnvFile writes key-value pairs to a .env file, sorted by key.
func WriteEnvFile(path string, env map[string]string) error {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, env[k])
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
