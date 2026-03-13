package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"

	"github.com/dargstack/dargstack/v4/internal/version"
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
	bgStarted  atomic.Bool
)

// BackgroundCheck starts a non-blocking update check.
func BackgroundCheck() {
	if version.Version == "dev" {
		return
	}
	bgOnce.Do(func() {
		bgStarted.Store(true)
		go func() {
			result, _ := checkLatest()
			bgResultCh <- result
		}()
	})
}

// CollectBackgroundCheck retrieves the result of BackgroundCheck with a short timeout.
// Returns nil immediately when no check was started (e.g. skipped commands, dev builds).
func CollectBackgroundCheck() *CheckResult {
	if !bgStarted.Load() {
		return nil
	}
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	latest, found, err := updater.DetectLatest(ctx, selfupdate.NewRepositorySlug(githubOwner, githubRepo))
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

	if err := updater.UpdateTo(ctx, latest, exe); err != nil {
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
	req.Header.Set("User-Agent", "dargstack/"+version.Version)
	req.Header.Set("Accept", "application/vnd.github+json")

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

// cacheFilePath returns the path to the update-check cache file.
// Returns an empty string when the user cache directory is unavailable;
// callers must treat an empty return value as "caching disabled".
func cacheFilePath() string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		// Do not fall back to os.TempDir(): a shared temp directory allows
		// symlink/hardlink attacks and cross-user cache poisoning.
		return ""
	}
	return filepath.Join(dir, cacheFile)
}

func readCache() *CheckResult {
	path := cacheFilePath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
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
	path := cacheFilePath()
	if path == "" {
		return
	}
	dir := filepath.Dir(path)
	// Ensure the cache directory is private to this user.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	entry := cacheEntry{
		CheckedAt:  time.Now(),
		Available:  result.Available,
		NewVersion: result.NewVersion,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	// Atomic write: write to a temp file in the same directory and rename so
	// concurrent readers never see a partial file and symlink attacks are avoided.
	tmp, err := os.CreateTemp(dir, ".dargstack-update-*")
	if err != nil {
		return
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() // clean up if rename doesn't run
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	_ = os.Rename(tmpPath, path)
}
