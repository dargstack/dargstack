package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"

	"github.com/dargstack/dargstack/internal/version"
)

const (
	githubOwner = "dargstack"
	githubRepo  = "dargstack"
	cacheFile   = ".dargstack-update-check"
	cacheTTL    = 24 * time.Hour
)

// CheckResult holds the outcome of an update check.
type CheckResult struct {
	Available  bool
	NewVersion string
}

var (
	bgResultCh = make(chan *CheckResult, 1)
	bgOnce     sync.Once
)

// BackgroundCheck starts a non-blocking update check.
func BackgroundCheck() {
	if version.Version == "dev" {
		return
	}
	bgOnce.Do(func() {
		go func() {
			result, _ := checkLatest()
			bgResultCh <- result
		}()
	})
}

// CollectBackgroundCheck retrieves the result of BackgroundCheck with a short timeout.
func CollectBackgroundCheck() *CheckResult {
	select {
	case result := <-bgResultCh:
		return result
	case <-time.After(50 * time.Millisecond):
		return nil
	}
}

// PrintUpdateNotice prints a notice if a newer version is available.
func PrintUpdateNotice(result *CheckResult) {
	if result == nil || !result.Available {
		return
	}
	fmt.Fprintf(os.Stderr, "\n  A new version of dargstack is available: %s -> %s\n", version.Version, result.NewVersion)
	fmt.Fprintf(os.Stderr, "  Run `dargstack update --self` to update.\n\n")
}

// SelfUpdate downloads and replaces the current binary with the latest release.
func SelfUpdate() error {
	if version.Version == "dev" {
		return fmt.Errorf("cannot self-update a development build")
	}

	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return fmt.Errorf("create update source: %w", err)
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{Source: source})
	if err != nil {
		return fmt.Errorf("create updater: %w", err)
	}

	latest, found, err := updater.DetectLatest(context.Background(), selfupdate.NewRepositorySlug(githubOwner, githubRepo))
	if err != nil {
		return fmt.Errorf("detect latest release: %w", err)
	}
	if !found {
		return fmt.Errorf("no releases found")
	}

	current, err := semver.NewVersion(version.Version)
	if err != nil {
		return fmt.Errorf("parse current version: %w", err)
	}

	if !latest.GreaterThan(current.String()) {
		fmt.Printf("Already at latest version %s\n", version.Version)
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable path: %w", err)
	}

	if err := updater.UpdateTo(context.Background(), latest, exe); err != nil {
		return fmt.Errorf("update: %w", err)
	}

	fmt.Printf("Updated to %s\n", latest.Version())
	return nil
}

func checkLatest() (*CheckResult, error) {
	if cached := readCache(); cached != nil {
		return cached, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", githubOwner, githubRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	latestTag := strings.TrimPrefix(release.TagName, "v")
	result := &CheckResult{NewVersion: latestTag}

	current, err := semver.NewVersion(version.Version)
	if err != nil {
		return result, nil
	}
	latestVer, err := semver.NewVersion(latestTag)
	if err != nil {
		return result, nil
	}

	result.Available = latestVer.GreaterThan(current)
	writeCache(result)
	return result, nil
}

type cacheEntry struct {
	CheckedAt  time.Time `json:"checked_at"`
	Available  bool      `json:"available"`
	NewVersion string    `json:"new_version"`
}

func cacheFilePath() string {
	dir, _ := os.UserCacheDir()
	if dir == "" {
		dir = os.TempDir()
	}
	return fmt.Sprintf("%s/%s", dir, cacheFile)
}

func readCache() *CheckResult {
	data, err := os.ReadFile(cacheFilePath())
	if err != nil {
		return nil
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}

	if time.Since(entry.CheckedAt) > cacheTTL {
		return nil
	}

	return &CheckResult{Available: entry.Available, NewVersion: entry.NewVersion}
}

func writeCache(result *CheckResult) {
	entry := cacheEntry{
		CheckedAt:  time.Now(),
		Available:  result.Available,
		NewVersion: result.NewVersion,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_ = os.WriteFile(cacheFilePath(), data, 0o644)
}
