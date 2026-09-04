package update

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"

	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/sudo"
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
		_, _ = lipgloss.Println(logger.StyleInfo.Render(fmt.Sprintf("Already at latest version %s", currentVersion())))
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable path: %w", err)
	}

	// Replacing the binary means creating and renaming files in its directory, so directory write access is what decides whether elevation is needed, not write access to the binary itself.
	if canWriteDir(filepath.Dir(exe)) {
		err = applyUpdate(ctx, updater, latest, exe)
	} else {
		err = applyUpdateElevated(ctx, updater, latest, exe)
	}
	if err != nil {
		return err
	}

	logger.Success(fmt.Sprintf("Updated to %s", latest.Version()))
	return nil
}

// applyUpdate downloads the release and installs it over target.
func applyUpdate(ctx context.Context, updater *selfupdate.Updater, latest *selfupdate.Release, target string) error {
	if err := updater.UpdateTo(ctx, latest, target); err != nil {
		if errors.Is(err, selfupdate.ErrChecksumValidationFailed) {
			return fmt.Errorf("update failed: checksum verification error, the release binary may be compromised: %w", err)
		}
		return fmt.Errorf("update: %w", err)
	}
	return nil
}

// applyUpdateElevated installs the new binary when its directory is writable only by root.
// Download and checksum validation still run unprivileged against a staging copy; only the final swap is elevated, so the release archive is never fetched or unpacked as root.
func applyUpdateElevated(ctx context.Context, updater *selfupdate.Updater, latest *selfupdate.Release, exe string) error {
	dir := filepath.Dir(exe)
	if !sudo.Available() {
		return fmt.Errorf("update: %s is not writable by the current user; re-run with elevated privileges", dir)
	}

	// MkdirTemp creates the directory with mode 0700, so no other user can swap the staged binary out before it is installed.
	stageDir, err := os.MkdirTemp("", "dargstack-update-")
	if err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(stageDir) }()

	// The staging file has to carry the binary's own name because the updater picks the matching entry out of the release archive by it.
	// It also has to exist already, because the updater renames the file it replaces out of the way before moving the new one in.
	stage := filepath.Join(stageDir, filepath.Base(exe))
	if err := os.WriteFile(stage, nil, 0o600); err != nil {
		return fmt.Errorf("create staging file: %w", err)
	}
	if err := applyUpdate(ctx, updater, latest, stage); err != nil {
		return err
	}

	if err := sudo.Prewarm(fmt.Sprintf("Updating dargstack requires write access to %s.", dir)); err != nil {
		return fmt.Errorf("sudo authentication failed: %w", err)
	}

	// Install into the target directory first and rename within it afterwards: that rename is atomic, and unlike writing over the binary in place it cannot fail with ETXTBSY while that same binary is the running process.
	incoming := filepath.Join(dir, "."+filepath.Base(exe)+".new")
	if err := runSudo("install", "-m", executableMode(exe), stage, incoming); err != nil {
		return err
	}
	if err := runSudo("mv", incoming, exe); err != nil {
		_ = runSudo("rm", "-f", incoming)
		return err
	}
	return nil
}

// executableMode returns the permission bits of the binary being replaced, as an octal string for install(1).
// It falls back to 0755 when the binary cannot be stat'ed, which is what a released binary is installed with anyway.
func executableMode(exe string) string {
	fi, err := os.Stat(exe)
	if err != nil {
		return "0755"
	}
	return fmt.Sprintf("%04o", fi.Mode().Perm())
}

// runSudo runs one command under sudo, folding its stderr into the returned error.
// Credentials are normally warmed by sudo.Prewarm beforehand, but the terminal stays connected so that a sudo configured without any credential caching can still prompt here.
func runSudo(args ...string) error {
	cmd := exec.Command("sudo", args...)
	var stderr bytes.Buffer
	cmd.Stdin = os.Stdin
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// canWriteDir reports whether the current user may create files in dir.
// It creates and removes a temporary file rather than inspecting permission bits, because that is the very operation the update performs and it accounts for ACLs and read-only mounts too.
func canWriteDir(dir string) bool {
	f, err := os.CreateTemp(dir, ".dargstack-write-check-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
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

	// Compare base versions (without pre-release) so a pre-release of a future version (e.g. 4.5.1-0.timestamp-commit) is not incorrectly flagged as needing an update to an earlier release (e.g. 4.4.0).
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
// Returns an empty string when the user cache directory is unavailable; callers must treat an empty return value as "caching disabled".
func cacheFilePath() string {
	dir, err := cacheDirFunc()
	if err != nil || dir == "" {
		// Do not fall back to os.TempDir(): a shared temp directory allows symlink/hardlink attacks and cross-user cache poisoning.
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

	// If the cache says an update was available, verify the current version isn't already at or past that version.
	// This prevents stale cache hits after a self-update (e.g. user was on 4.6.0, cache says 4.7.0 available, user updates to 4.7.0, but cache still reports Available=true).
	if entry.Available && entry.NewVersion != "" {
		cur, cerr := semver.NewVersion(strings.TrimPrefix(currentVersion(), "v"))
		cached, cachedErr := semver.NewVersion(entry.NewVersion)
		if cerr == nil && cachedErr == nil {
			if cur.GreaterThan(cached) || cur.Equal(cached) {
				return nil
			}
		}
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
	// Atomic write: write to a temp file in the same directory and rename so concurrent readers never see a partial file and symlink attacks are avoided.
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
