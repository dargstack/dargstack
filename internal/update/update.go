package update

import (
	"context"
	"encoding/json"
	"errors"
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

	"github.com/dargstack/dargstack/v4/internal/logger"
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
	bgComplete atomic.Bool

	// doHTTPRequest abstracts HTTP requests for testability.
	doHTTPRequest = defaultDoHTTPRequest

	// cacheDirFunc abstracts os.UserCacheDir for testability.
	cacheDirFunc = os.UserCacheDir

	// currentVersion returns the running version; overridden in tests.
	currentVersion = func() string { return version.Version }
)

func defaultDoHTTPRequest(req *http.Request) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

// resetBackgroundState resets the background check state for testing.
func resetBackgroundState() {
	bgOnce = sync.Once{}
	bgStarted.Store(false)
	bgComplete.Store(false)
	// Drain any leftover result.
	select {
	case <-bgResultCh:
	default:
	}
}

// BackgroundCheck starts a non-blocking update check.
func BackgroundCheck() {
	v := currentVersion()
	if v == "dev" || strings.HasSuffix(v, "+dirty") {
		return
	}
	bgOnce.Do(func() {
		bgStarted.Store(true)
		go func() {
			defer bgComplete.Store(true)
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
	current := strings.TrimPrefix(currentVersion(), "v")
	logger.L.Warn(fmt.Sprintf("A new version of dargstack is available: %s -> %s", current, result.NewVersion))
	logger.L.Warn("Run `dargstack update --self` to update.")
}

// SelfUpdate downloads and replaces the current binary with the latest release.
func SelfUpdate() error {
	if currentVersion() == "dev" {
		return fmt.Errorf("cannot self-update a development build")
	}

	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return fmt.Errorf("create update source: %w", err)
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source:    source,
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})
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

	current, err := semver.NewVersion(currentVersion())
	if err != nil {
		return fmt.Errorf("parse current version: %w", err)
	}

	if !latest.GreaterThan(current.String()) {
		fmt.Printf("Already at latest version %s\n", currentVersion())
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable path: %w", err)
	}

	if err := updater.UpdateTo(ctx, latest, exe); err != nil {
		if errors.Is(err, selfupdate.ErrChecksumValidationFailed) {
			return fmt.Errorf("update failed: checksum verification error — the release binary may be compromised: %w", err)
		}
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
	req.Header.Set("User-Agent", "dargstack/"+currentVersion())
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := doHTTPRequest(req)
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

	currentSemver, err := semver.NewVersion(currentVersion())
	if err != nil {
		return result, nil
	}
	latestVer, err := semver.NewVersion(latestTag)
	if err != nil {
		return result, nil
	}

	// Compare base versions (without pre-release) so a pre-release of a
	// future version (e.g. 4.5.1-0.timestamp-commit) is not incorrectly
	// flagged as needing an update to an earlier release (e.g. 4.4.0).
	currentBase := currentSemver.Original()
	if pre := currentSemver.Prerelease(); pre != "" {
		currentBase = fmt.Sprintf("%d.%d.%d", currentSemver.Major(), currentSemver.Minor(), currentSemver.Patch())
	}
	currentBaseVer, err := semver.NewVersion(currentBase)
	if err == nil && latestVer.GreaterThan(currentBaseVer) {
		result.Available = true
	}
	writeCache(result)
	return result, nil
}

type cacheEntry struct {
	Available  bool      `json:"available"`
	CheckedAt  time.Time `json:"checked_at"`
	NewVersion string    `json:"new_version"`
}

// cacheFilePath returns the path to the update-check cache file.
// Returns an empty string when the user cache directory is unavailable;
// callers must treat an empty return value as "caching disabled".
func cacheFilePath() string {
	dir, err := cacheDirFunc()
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
	if err := os.Rename(tmpPath, path); err != nil {
		// Windows can't rename over an existing file; remove and retry.
		_ = os.Remove(path)
		_ = os.Rename(tmpPath, path)
	}
}
