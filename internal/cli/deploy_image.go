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

	"gopkg.in/yaml.v3"

	"github.com/dargstack/dargstack/v4/internal/config"
	"github.com/dargstack/dargstack/v4/internal/docker"
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
	if cfg.Production.Tag != "latest" {
		return cfg.Production.Tag, nil
	}
	if !offline {
		if err := gitFetchTags(); err != nil {
			printWarning(fmt.Sprintf("Failed to fetch remote tags: %v", err))
		}
	}
	tag, err := latestGitTag(cfg.Production.Branch)
	if err != nil {
		return "", fmt.Errorf("resolve deploy tag from branch %q: %w — use --tag to set explicitly", cfg.Production.Branch, err)
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

	baseDir := config.DevDir(stackDir)

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
				printWarning(fmt.Sprintf("Service %q: directory not found at %s", name, svcDir))
				continue
			}
			contextPath = filepath.Join(svcDir, contextPath)
		}

		tag := fmt.Sprintf("%s/%s:development", cfg.Name, name)

		// Skip building if behavior.build.mode is "missing" and image already exists.
		if cfg.Behavior.Build != nil && cfg.Behavior.Build.Mode == "missing" && imageExists(executor, tag) {
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
		printInfo(fmt.Sprintf("Building %d image(s) in parallel: %s", len(tasks), joinNames(extractNames(tasks))))
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
			printSuccess(fmt.Sprintf(MsgBuiltImage, task.tag))
		}
	} else {
		printSuccess(fmt.Sprintf("Built %d image(s)", successCount))
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
// and prints an aggregate warning for any that are behind their remote.
func fetchAndWarnBehind(composeData []byte) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return
	}

	svcMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return
	}

	baseDir := config.DevDir(stackDir)

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

		contextPath := extractDargstackBuildContext(svc)
		if contextPath == "" {
			continue
		}

		if !filepath.IsAbs(contextPath) {
			svcDir := filepath.Join(baseDir, name)
			if _, err := os.Stat(svcDir); os.IsNotExist(err) {
				continue
			}
			contextPath = filepath.Join(svcDir, contextPath)
		}

		dirs = append(dirs, contextDir{name: name, path: contextPath})
	}

	if len(dirs) == 0 {
		return
	}

	// Fetch and check behind in parallel.
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
		return
	}

	// Sort for deterministic output.
	sort.Slice(behind, func(i, j int) bool { return behind[i].serviceName < behind[j].serviceName })

	parts := make([]string, len(behind))
	for i, b := range behind {
		parts[i] = fmt.Sprintf("%s (%s) — %d commit%s behind", b.serviceName, b.branch, b.behind, pluralS(b.behind))
	}
	printWarning(fmt.Sprintf("Local repos behind remote: %s", strings.Join(parts, ", ")))
}

// fetchAndCheckBehind runs `git fetch` in dir and returns how many commits
// the current branch is behind its upstream. Returns (0, "", err) on failure.
func fetchAndCheckBehind(dir string) (int, string, error) {
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
	branchOut, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return 0, "", nil
	}
	branch := strings.TrimSpace(string(branchOut))
	if branch == "HEAD" {
		// Detached HEAD — nothing to compare.
		return 0, "", nil
	}

	// Get upstream branch.
	upstreamOut, err := exec.Command("git", "rev-parse", "--abbrev-ref", branch+"@{upstream}").Output()
	if err != nil {
		// No upstream configured.
		return 0, "", nil
	}
	upstream := strings.TrimSpace(string(upstreamOut))

	// Count commits behind: `git rev-list COUNT..upstream --count`
	countOut, err := exec.Command("git", "rev-list", fmt.Sprintf("%s..%s", branch, upstream), "--count").Output()
	if err != nil {
		return 0, branch, nil
	}
	count := strings.TrimSpace(string(countOut))

	behind, err := strconv.Atoi(count)
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

// repoNameFromURL extracts the repository directory name from a git URL.
// It handles SSH (git@host:user/repo.git), HTTPS (https://host/user/repo.git),
// and git:// formats. The .git suffix is stripped if present.
func repoNameFromURL(url string) string {
	name := url
	// For SSH URLs like git@github.com:user/repo.git, take after the colon
	if idx := strings.LastIndex(name, ":"); idx >= 0 {
		name = name[idx+1:]
	}
	// Take the last path segment
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	// Strip .git suffix
	name = strings.TrimSuffix(name, ".git")
	return name
}

// extractDargstackGitLabel returns the git URL from a
// deploy.labels.dargstack.development.git label, or "" if not present.
func extractDargstackGitLabel(svc map[string]interface{}) string {
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
		if git, ok := v["dargstack.development.git"].(string); ok {
			return git
		}
	case []interface{}:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			if strings.HasPrefix(s, "dargstack.development.git=") {
				return strings.TrimPrefix(s, "dargstack.development.git=")
			}
		}
	}
	return ""
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
	// .build label takes precedence
	if ctx := extractDargstackBuildContext(svc); ctx != "" {
		return ctx
	}

	// Fall back to .git label: derive context from cloned repo directory
	gitURL := extractDargstackGitLabel(svc)
	if gitURL == "" {
		return ""
	}

	repoName := repoNameFromURL(gitURL)
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
		printWarning(fmt.Sprintf("Container cleanup failed: %v", err))
		return
	}

	imageOut, err := executor.Run("image", "prune", "-af")
	if err != nil {
		printWarning(fmt.Sprintf("Image cleanup failed: %v", err))
		return
	}

	printSuccess(fmt.Sprintf(
		"Cleanup complete. Containers: %s | Images: %s",
		strings.TrimSpace(containerOut),
		strings.TrimSpace(imageOut),
	))
}
