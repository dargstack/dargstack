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

	"go.yaml.in/yaml/v3"

	"github.com/dargstack/dargstack/v4/internal/config"
	"github.com/dargstack/dargstack/v4/internal/docker"
	"github.com/dargstack/dargstack/v4/internal/giturl"
	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/prompt"
)

// buildStatus tracks the live status of parallel builds for non-verbose output.
type buildStatus struct {
	mu     sync.Mutex
	tasks  []buildTask
	status map[string]string // name -> current status string
	lines  int               // number of lines printed
}

func newBuildStatus(tasks []buildTask) *buildStatus {
	return &buildStatus{
		tasks:  tasks,
		status: make(map[string]string, len(tasks)),
	}
}

// printAll prints or overwrites all status lines.
func (bs *buildStatus) printAll() {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if bs.lines > 0 {
		// Move cursor up to the first status line.
		fmt.Printf("\033[%dA", bs.lines)
	}
	for _, t := range bs.tasks {
		s := bs.status[t.name]
		if s == "" {
			s = "building..."
		}
		fmt.Printf("  [%s] %s\033[K\n", t.name, s)
	}
}

// set updates a task's status and redraws.
func (bs *buildStatus) set(name, status string) {
	bs.mu.Lock()
	bs.status[name] = status
	bs.lines = len(bs.tasks)
	bs.mu.Unlock()
	bs.printAll()
}

// done clears the status lines and returns to normal output.
func (bs *buildStatus) done() {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	// Clear status lines.
	for i := 0; i < bs.lines; i++ {
		fmt.Print("\033[2K\r\n")
	}
	// Move cursor back up.
	fmt.Printf("\033[%dA", bs.lines)
	bs.lines = 0
}

func resolveDeployTag() (string, error) {
	if deployTag != "" {
		return deployTag, nil
	}
	if cfg.Environment.Production.Tag != "" {
		return cfg.Environment.Production.Tag, nil
	}
	if !offline {
		if err := gitFetchTags(); err != nil {
			logger.L.Warn(fmt.Sprintf("Failed to fetch remote tags: %v", err))
		}
	}
	tag, err := latestGitTag(cfg.Environment.Production.Branch)
	if err != nil {
		return "", fmt.Errorf("resolve deploy tag from branch %q: %w — use --tag to set explicitly", cfg.Environment.Production.Branch, err)
	}
	return tag, nil
}

func gitFetchTags() error {
	cmd := exec.Command("git", "fetch", "--tags", "origin")
	cmd.Dir = stackDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func latestGitTag(branch string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0", branch)
	cmd.Dir = stackDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
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
		if cfg.Runtime.Build.Mode == config.BuildMissing && imageExists(executor, tag) {
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

	var bs *buildStatus
	if !verbose {
		bs = newBuildStatus(tasks)
		defer bs.done()
		bs.printAll()
	}

	// Run builds in parallel.
	var wg sync.WaitGroup
	var mu sync.Mutex
	var buildErrs []string
	successCount := 0

	for _, task := range tasks {
		wg.Add(1)
		go func(t buildTask) {
			defer wg.Done()

			if err := docker.StackBuild(executor, t.name, verbose, t.contextPath, "development", t.tag); err != nil {
				if bs != nil {
					bs.set(t.name, "✗ failed")
				}
				mu.Lock()
				buildErrs = append(buildErrs, fmt.Sprintf("build %s: %v", t.name, err))
				mu.Unlock()
				return
			}

			if bs != nil {
				bs.set(t.name, "✓ built")
			}
			mu.Lock()
			successCount++
			mu.Unlock()
		}(task)
	}

	wg.Wait()

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

// fetchAndWarnBehind fetches all git repos used as build contexts in parallel
// and returns info for any that are behind their remote (caller prints the warning).
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

	// Fetch and check behind in parallel.
	logger.L.Info(fmt.Sprintf("Checking %d repo%s for remote changes...", len(dirs), pluralS(len(dirs))))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var behind []gitBehindInfo

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
		parts[i] = fmt.Sprintf("%s (%s) — %d commit%s behind", b.serviceName, b.branch, b.behind, pluralS(b.behind))
	}
	logger.L.Warn(fmt.Sprintf("Local repos behind remote: %s", strings.Join(parts, ", ")))
}

// fetchAndCheckBehind runs `git fetch` in dir and returns how many commits
// the current branch is behind its upstream. Returns (0, "", err) on failure.
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
		// Detached HEAD — nothing to compare.
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

func imageExists(executor *docker.Executor, tag string) bool {
	_, err := executor.Run("image", "inspect", "--format", "{{.ID}}", tag)
	return err == nil
}

// extractDargstackBuildContext returns the build context from a
// deploy.labels.dargstack.development.build label, or "" if not present.
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
// If not present, falls back to `dargstack.development.git` and derives
// the context from the cloned repo directory (sibling of the stack directory).
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
	parentDir := filepath.Dir(stackDir)
	return filepath.Join(parentDir, repoName)
}

// offerRuntimeCleanup prompts to remove stopped containers and then unused images.
func offerRuntimeCleanup(executor *docker.Executor) {
	ok, err := prompt.Confirm("Remove stopped containers and unused images now?", false)
	if err != nil || !ok {
		return
	}

	containerOut, err := executor.Run("container", "prune", "-f")
	if err != nil {
		logger.L.Warn(fmt.Sprintf("Container cleanup failed: %v", err))
		return
	}

	imageOut, err := executor.Run("image", "prune", "-af")
	if err != nil {
		logger.L.Warn(fmt.Sprintf("Image cleanup failed: %v", err))
		return
	}

	logger.Success(fmt.Sprintf(
		"Cleanup complete. Containers: %s | Images: %s",
		strings.TrimSpace(containerOut),
		strings.TrimSpace(imageOut),
	))
}
