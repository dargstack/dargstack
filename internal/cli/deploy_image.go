package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"charm.land/huh/v2/spinner"
	"github.com/Masterminds/semver/v3"
	"go.yaml.in/yaml/v3"

	"github.com/dargstack/dargstack/v4/internal/config"
	"github.com/dargstack/dargstack/v4/internal/docker"
	"github.com/dargstack/dargstack/v4/internal/giturl"
	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/prompt"
)

func parseMajorVersion(tag string) (int, error) {
	tag = strings.TrimPrefix(tag, "v")
	parts := strings.SplitN(tag, ".", 2)
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("parse major version from %q: %w", tag, err)
	}
	return major, nil
}

func currentTag() string {
	cmd := exec.Command("git", "describe", "--tags", "--exact-match", "HEAD")
	cmd.Dir = stackDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func resolveDeployTag() (string, error) {
	current := currentTag()
	currentMajor, majorErr := parseMajorVersion(current)
	currentDetermined := current != "" && majorErr == nil

	var targetTag string
	if deployTag != "" {
		targetTag = deployTag
	} else if cfg.Environment.Production.Tag != "" {
		targetTag = cfg.Environment.Production.Tag
	}

	if targetTag != "" {
		targetMajor, err := parseMajorVersion(targetTag)
		if err != nil {
			// Non-semver tag (e.g. "latest"): no major to guard on, deploy as-is.
			return targetTag, nil
		}

		if !currentDetermined {
			if !deployMajor {
				return "", fmt.Errorf("current major version could not be determined (no tagged commit checked out at HEAD): pass --major to confirm this deploy")
			}
			return targetTag, nil
		}

		if targetMajor == currentMajor {
			return targetTag, nil
		}

		if !deployMajor {
			return "", fmt.Errorf("deploying %s changes the major version from %s (v%d -> v%d): use --major to confirm", targetTag, current, currentMajor, targetMajor)
		}

		if abs(targetMajor-currentMajor) > 1 {
			nextMajor := currentMajor + signInt(targetMajor-currentMajor)
			return "", fmt.Errorf("deploying %s skips major version v%d (currently on %s): deploy v%d first", targetTag, nextMajor, current, nextMajor)
		}

		return targetTag, nil
	}

	// Auto-resolution
	if !offline {
		if err := gitFetchOrigin(); err != nil {
			logger.L.Warn(fmt.Sprintf("Failed to fetch from origin: %v", err))
		}
	}

	if !currentDetermined {
		if !deployMajor {
			return "", fmt.Errorf("current major version could not be determined (no tagged commit checked out at HEAD): pass --major to confirm this deploy")
		}
		tag, err := latestGitTag(-1)
		if err != nil {
			return "", fmt.Errorf("resolve deploy tag from branch %q: %w", cfg.Environment.Production.Branch, err)
		}
		return tag, nil
	}

	targetMajor := currentMajor
	if deployMajor {
		targetMajor = currentMajor + 1
	}

	tag, err := latestGitTag(targetMajor)
	if err != nil {
		if deployMajor {
			return "", fmt.Errorf("no v%d.x.x tag found: version v%d has not been released yet", targetMajor, targetMajor)
		}
		return "", fmt.Errorf("resolve deploy tag from branch %q: %w, use --tag to set explicitly", cfg.Environment.Production.Branch, err)
	}
	return tag, nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func signInt(x int) int {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}

func gitFetchOrigin() error {
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = stackDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func latestGitTag(targetMajor int) (string, error) {
	cmd := exec.Command("git", "tag", "--list")
	cmd.Dir = stackDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("list git tags: %s", strings.TrimSpace(string(out)))
	}

	var candidates []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		major, parseErr := parseMajorVersion(line)
		if parseErr != nil {
			continue
		}
		if targetMajor >= 0 && major != targetMajor {
			continue
		}
		candidates = append(candidates, line)
	}

	if len(candidates) == 0 {
		if targetMajor >= 0 {
			return "", fmt.Errorf("no tag found matching major %d", targetMajor)
		}
		return "", fmt.Errorf("no tags found in repository")
	}

	sort.Slice(candidates, func(i, j int) bool {
		return compareSemver(candidates[i], candidates[j]) > 0
	})

	return candidates[0], nil
}

// compareSemver compares two tags by semver precedence, ignoring an optional "v" prefix on either side.
// A plain string or git version-sort comparison gets this wrong: it buckets "v"-prefixed and bare-digit tags separately regardless of actual version, and sorts multi-digit segments lexicographically (making "v2.10.0" sort below "v2.9.0" because '1' < '9').
// A naive dot-split comparator also has no notion of prerelease precedence, so a tag like "v15.0.0-beta.5" would sort above "v15.0.0".
func compareSemver(a, b string) int {
	av, aErr := semver.NewVersion(a)
	bv, bErr := semver.NewVersion(b)
	if aErr != nil || bErr != nil {
		// Neither tag reaches here unless it already passed parseMajorVersion, so this only covers a major-only parse succeeding while the full semver parse still fails.
		if aErr != nil && bErr != nil {
			return 0
		}
		if aErr != nil {
			return -1
		}
		return 1
	}
	return av.Compare(bv)
}

// gitWorkingTreeDirty reports whether dir has uncommitted changes to tracked files.
// Untracked files (e.g. generated artifacts) are ignored: git itself refuses a checkout that would clobber one, so that failure surfaces on its own when it matters.
func gitWorkingTreeDirty(dir string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=no")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func gitCheckout(dir, ref string) error {
	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("invalid git ref %q", ref)
	}
	cmd := exec.Command("git", "checkout", "--detach", ref)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// checkoutDeployTag resolves the production deploy tag (via resolveDeployTag) and checks it out in stackDir so the compose files that get deployed actually match the tagged release, rather than whatever happened to be on disk.
// The checkout is left in place (detached at the tag) after deploy.
// When offline, it only resolves the tag from local state without checking out: the working tree is left as-is.
func checkoutDeployTag() (string, error) {
	tag, err := resolveDeployTag()
	if err != nil {
		return "", err
	}

	if offline {
		logger.L.Info(fmt.Sprintf("Using local tag %q for production deploy (--offline)", tag))
		return tag, nil
	}

	dirty, err := gitWorkingTreeDirty(stackDir)
	if err != nil {
		return "", fmt.Errorf("check git working tree status: %w", err)
	}
	if dirty {
		return "", fmt.Errorf("stack directory has uncommitted changes to tracked files: commit or stash them before deploying to production")
	}

	if err := gitCheckout(stackDir, tag); err != nil {
		return "", fmt.Errorf("checkout tag %q: %w", tag, err)
	}
	logger.L.Info(fmt.Sprintf("Checked out tag %q for production deploy", tag))

	return tag, nil
}

// buildTask holds the parameters for a single image build.
type buildTask struct {
	name        string
	contextPath string
	tag         string
}

// autoBuildServices builds images for services that have a dargstack.development.build label.
// When behavior.build.mode is "missing", images are only built if they don't already exist locally.
// When behavior.build.mode is "always" (default), images are always rebuilt.
// Builds run in parallel; output is suppressed unless verbose or a build fails.
func autoBuildServices(executor *docker.Executor, composeData []byte) error {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return err
	}

	svcMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return nil
	}

	baseDir := cfg.DevDir()

	// Collect build tasks in deterministic order.
	var tasks []buildTask
	for name, def := range svcMap {
		svc, ok := def.(map[string]interface{})
		if !ok {
			continue
		}

		contextPath := extractDargstackBuildContext(svc)
		if contextPath == "" {
			continue
		}

		if !filepath.IsAbs(contextPath) {
			svcDir := filepath.Join(baseDir, name)
			if _, err := os.Stat(svcDir); os.IsNotExist(err) {
				logger.L.Warn(fmt.Sprintf("Service %q: directory not found at %s", name, svcDir))
				continue
			}
			contextPath = filepath.Join(svcDir, contextPath)
		}

		tag := fmt.Sprintf("%s/%s:development", cfg.Metadata.Name, name)

		// Skip building if behavior.build.mode is "missing" and image already exists.
		if cfg.Runtime.Build.Mode == config.BuildMissing && docker.ImageExistsLocally(executor, tag) {
			continue
		}

		tasks = append(tasks, buildTask{name: name, contextPath: contextPath, tag: tag})
	}

	if len(tasks) == 0 {
		return nil
	}

	// Sort for deterministic output.
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].name < tasks[j].name })

	if verbose {
		logger.L.Info(fmt.Sprintf("Building %d image(s) in parallel: %s", len(tasks), joinNames(extractNames(tasks))))
	}

	var buildErrs []string
	var mu sync.Mutex
	successCount := 0

	runBuilds := func() {
		var wg sync.WaitGroup
		for _, task := range tasks {
			wg.Add(1)
			go func(t buildTask) {
				defer wg.Done()

				if err := docker.StackBuild(executor, t.name, verbose, t.contextPath, "development", t.tag); err != nil {
					mu.Lock()
					buildErrs = append(buildErrs, fmt.Sprintf("build %s: %v", t.name, err))
					mu.Unlock()
					return
				}

				mu.Lock()
				successCount++
				mu.Unlock()
			}(task)
		}
		wg.Wait()
	}

	if !verbose && !noInteraction {
		err := spinner.New().
			Title("Building images").
			Action(func() {
				runBuilds()
			}).
			Run()
		if err != nil {
			return err
		}
	} else {
		runBuilds()
	}

	if len(buildErrs) > 0 {
		return fmt.Errorf("build errors:\n  %s", joinNamesWithNewline(buildErrs))
	}

	if verbose {
		for _, task := range tasks {
			logger.Success(fmt.Sprintf(MsgBuiltImage, task.tag))
		}
	} else {
		logger.Success(fmt.Sprintf("Built %d image(s)", successCount))
	}

	return nil
}

func extractNames(tasks []buildTask) []string {
	names := make([]string, len(tasks))
	for i, t := range tasks {
		names[i] = t.name
	}
	return names
}

// gitBehindInfo holds the result of checking if a repo is behind its remote.
type gitBehindInfo struct {
	serviceName string
	behind      int
	branch      string
}

// fetchAndWarnBehind fetches all git repos used as build contexts in parallel and returns info for any that are behind their remote (caller prints the warning).
func fetchAndWarnBehind(composeData []byte) []gitBehindInfo {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil
	}

	svcMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Collect build context directories.
	type contextDir struct {
		name string
		path string
	}
	var dirs []contextDir

	for name, def := range svcMap {
		svc, ok := def.(map[string]interface{})
		if !ok {
			continue
		}

		contextPath := resolveBuildContext(svc, stackDir)
		if contextPath == "" {
			continue
		}

		dirs = append(dirs, contextDir{name: name, path: contextPath})
	}

	if len(dirs) == 0 {
		return nil
	}

	var behind []gitBehindInfo
	checkAction := func() {
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, d := range dirs {
			wg.Add(1)
			go func(cd contextDir) {
				defer wg.Done()

				behindCount, branch, err := fetchAndCheckBehind(cd.path)
				if err != nil || behindCount == 0 {
					return
				}

				mu.Lock()
				behind = append(behind, gitBehindInfo{serviceName: cd.name, behind: behindCount, branch: branch})
				mu.Unlock()
			}(d)
		}

		wg.Wait()
	}

	if !noInteraction {
		_ = spinner.New().
			Title("Checking repositories for updates").
			Action(checkAction).
			Run()
	} else {
		checkAction()
	}

	if len(behind) == 0 && len(dirs) > 0 {
		logger.Success("All repositories are up to date")
		return nil
	}
	if len(behind) == 0 {
		return nil
	}

	// Sort for deterministic output.
	sort.Slice(behind, func(i, j int) bool { return behind[i].serviceName < behind[j].serviceName })
	return behind
}

// printBehindWarning prints the aggregate behind-remote warning.
func printBehindWarning(behind []gitBehindInfo) {
	if len(behind) == 0 {
		return
	}
	parts := make([]string, len(behind))
	for i, b := range behind {
		parts[i] = fmt.Sprintf("  %s (%s): %d commit%s behind", b.serviceName, b.branch, b.behind, pluralS(b.behind))
	}
	logger.L.Warn("Local repos behind remote:\n" + strings.Join(parts, "\n"))
}

// fetchAndCheckBehind runs `git fetch` in dir and returns how many commits the current branch is behind its upstream.
// Returns (0, "", err) on failure.
func fetchAndCheckBehind(dir string) (behind int, branch string, err error) {
	// Check if it's a git repo.
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		return 0, "", nil
	}

	// Fetch.
	cmd := exec.Command("git", "fetch")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return 0, "", fmt.Errorf("git fetch in %s: %s: %w", dir, strings.TrimSpace(string(out)), err)
	}

	// Get current branch.
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = dir
	branchOut, err := branchCmd.Output()
	if err != nil {
		return 0, "", nil
	}
	branch = strings.TrimSpace(string(branchOut))
	if branch == "HEAD" {
		// Detached HEAD: nothing to compare.
		return 0, "", nil
	}

	// Get upstream branch.
	upstreamCmd := exec.Command("git", "rev-parse", "--abbrev-ref", branch+"@{upstream}")
	upstreamCmd.Dir = dir
	upstreamOut, err := upstreamCmd.Output()
	if err != nil {
		// No upstream configured.
		return 0, "", nil
	}
	upstream := strings.TrimSpace(string(upstreamOut))

	// Count commits behind: `git rev-list COUNT..upstream --count`
	countCmd := exec.Command("git", "rev-list", fmt.Sprintf("%s..%s", branch, upstream), "--count")
	countCmd.Dir = dir
	countOut, err := countCmd.Output()
	if err != nil {
		return 0, branch, nil
	}
	count := strings.TrimSpace(string(countOut))

	behind, err = strconv.Atoi(count)
	if err != nil {
		return 0, branch, nil
	}
	return behind, branch, nil
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// extractDargstackBuildContext returns the build context from a deploy.labels.dargstack.development.build label, or "" if not present.
func extractDargstackBuildContext(svc map[string]interface{}) string {
	deploy, ok := svc["deploy"].(map[string]interface{})
	if !ok {
		return ""
	}
	labels, ok := deploy["labels"]
	if !ok {
		return ""
	}
	switch v := labels.(type) {
	case map[string]interface{}:
		if ctx, ok := v["dargstack.development.build"].(string); ok {
			return ctx
		}
	case []interface{}:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			if strings.HasPrefix(s, "dargstack.development.build=") {
				return strings.TrimPrefix(s, "dargstack.development.build=")
			}
		}
	}
	return ""
}

// resolveBuildContext returns the build context for a service.
// It checks for a `dargstack.development.build` label first.
// If not present, falls back to `dargstack.development.git.ssh`/`dargstack.development.git.https` and derives the context from the cloned repo directory (sibling of the stack directory).
// Returns "" if neither label is set.
func resolveBuildContext(svc map[string]interface{}, stackDir string) string {
	if ctx := extractDargstackBuildContext(svc); ctx != "" {
		return ctx
	}

	gitURL := giturl.ExtractFromService(svc, "")
	if !gitURL.IsSet() {
		return ""
	}

	repoName := giturl.RepoNameFromURL(gitURL.Primary())
	parentDir := giturl.SiblingParentDir(stackDir)
	return filepath.Join(parentDir, repoName)
}

// offerRuntimeCleanup prompts to remove stopped containers and then unused images.
func offerRuntimeCleanup(executor *docker.Executor) {
	ok, err := prompt.Confirm("Remove stopped containers and unused images now?", false)
	if err != nil || !ok {
		return
	}

	var containerOut, imageOut string
	var containerErr, imageErr error

	_ = spinner.New().
		Title("Cleaning up").
		Action(func() {
			containerOut, containerErr = executor.Run("container", "prune", "-f")
			if containerErr != nil {
				return
			}
			imageOut, imageErr = executor.Run("image", "prune", "-af")
		}).
		Run()

	if containerErr != nil {
		logger.L.Warn(fmt.Sprintf("Container cleanup failed: %v", containerErr))
		return
	}
	if imageErr != nil {
		logger.L.Warn(fmt.Sprintf("Image cleanup failed: %v", imageErr))
		return
	}

	logger.Success(fmt.Sprintf(
		"Cleanup complete. Containers: %s | Images: %s",
		strings.TrimSpace(containerOut),
		strings.TrimSpace(imageOut),
	))
}
