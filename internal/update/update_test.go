package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testSetup(t *testing.T) func(t *testing.T) {
	t.Helper()
	origDoHTTPRequest := doHTTPRequest
	origCacheDirFunc := cacheDirFunc
	origCurrentVersion := currentVersion
	resetBackgroundState()
	return func(t *testing.T) {
		// Wait for the background goroutine to finish before restoring
		// package-level variables, preventing data races.
		for i := 0; i < 100; i++ {
			if bgComplete.Load() {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		doHTTPRequest = origDoHTTPRequest
		cacheDirFunc = origCacheDirFunc
		currentVersion = origCurrentVersion
		resetBackgroundState()
	}
}

// -------------------------------------------------------------------
// checkLatest tests (HTTP mocking)
// -------------------------------------------------------------------

func redirectToServer(serverURL string) func(req *http.Request) (*http.Response, error) {
	host := serverURL[len("http://"):]
	return func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = host
		return http.DefaultClient.Do(req)
	}
}

func TestCheckLatest_NewerVersion(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	currentVersion = func() string { return "1.0.0" }
	cacheDirFunc = func() (string, error) { return t.TempDir(), nil }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("missing Accept header, got: %q", r.Header.Get("Accept"))
		}
		if !strings.Contains(r.Header.Get("User-Agent"), "dargstack/") {
			t.Errorf("missing User-Agent, got: %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name":"v2.0.0"}`)
	}))
	defer server.Close()

	doHTTPRequest = redirectToServer(server.URL)

	result, err := checkLatest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Available {
		t.Error("expected update available")
	}
	if result.NewVersion != "2.0.0" {
		t.Errorf("expected NewVersion=2.0.0, got %q", result.NewVersion)
	}
}

func TestCheckLatest_SameVersion(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	currentVersion = func() string { return "1.5.0" }
	cacheDirFunc = func() (string, error) { return t.TempDir(), nil }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name":"v1.5.0"}`)
	}))
	defer server.Close()

	doHTTPRequest = redirectToServer(server.URL)

	result, err := checkLatest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Available {
		t.Error("expected no update available")
	}
	if result.NewVersion != "1.5.0" {
		t.Errorf("expected NewVersion=1.5.0, got %q", result.NewVersion)
	}
}

func TestCheckLatest_OlderVersion(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	currentVersion = func() string { return "3.0.0" }
	cacheDirFunc = func() (string, error) { return t.TempDir(), nil }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name":"v2.0.0"}`)
	}))
	defer server.Close()

	doHTTPRequest = redirectToServer(server.URL)

	result, err := checkLatest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Available {
		t.Error("expected no update available when remote is older")
	}
	if result.NewVersion != "2.0.0" {
		t.Errorf("expected NewVersion=2.0.0, got %q", result.NewVersion)
	}
}

func TestCheckLatest_HTTPError(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	currentVersion = func() string { return "1.0.0" }
	cacheDirFunc = func() (string, error) { return t.TempDir(), nil }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	doHTTPRequest = redirectToServer(server.URL)

	_, err := checkLatest()
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error to contain status code, got: %v", err)
	}
}

func TestCheckLatest_NetworkError(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	currentVersion = func() string { return "1.0.0" }
	cacheDirFunc = func() (string, error) { return t.TempDir(), nil }

	doHTTPRequest = func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("connection refused")
	}

	_, err := checkLatest()
	if err == nil {
		t.Fatal("expected error for network failure")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("expected 'connection refused', got: %v", err)
	}
}

func TestCheckLatest_InvalidJSON(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	currentVersion = func() string { return "1.0.0" }
	cacheDirFunc = func() (string, error) { return t.TempDir(), nil }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{invalid json}`)
	}))
	defer server.Close()

	doHTTPRequest = redirectToServer(server.URL)

	_, err := checkLatest()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCheckLatest_CacheHit(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	currentVersion = func() string { return "1.0.0" }
	tmpDir := t.TempDir()
	cacheDirFunc = func() (string, error) { return tmpDir, nil }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name":"v2.0.0"}`)
	}))
	defer server.Close()

	callCount := 0
	doHTTPRequest = func(req *http.Request) (*http.Response, error) {
		callCount++
		req.URL.Scheme = "http"
		req.URL.Host = server.URL[len("http://"):]
		return http.DefaultClient.Do(req)
	}

	// First call should hit the network.
	result1, err := checkLatest()
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 HTTP call, got %d", callCount)
	}

	// Second call should use cache.
	result2, err := checkLatest()
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 HTTP call (cached), got %d", callCount)
	}
	if result2.NewVersion != result1.NewVersion {
		t.Errorf("cache mismatch: %q vs %q", result2.NewVersion, result1.NewVersion)
	}
}

func TestCheckLatest_CacheDisabled(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	currentVersion = func() string { return "1.0.0" }
	cacheDirFunc = func() (string, error) { return "", fmt.Errorf("no cache dir") }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name":"v2.0.0"}`)
	}))
	defer server.Close()

	callCount := 0
	doHTTPRequest = func(req *http.Request) (*http.Response, error) {
		callCount++
		req.URL.Scheme = "http"
		req.URL.Host = server.URL[len("http://"):]
		return http.DefaultClient.Do(req)
	}

	// Both calls should hit the network since cache is disabled.
	for i := 0; i < 2; i++ {
		_, err := checkLatest()
		if err != nil {
			t.Fatalf("call %d failed: %v", i+1, err)
		}
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls (no cache), got %d", callCount)
	}
}

func TestCheckLatest_DevVersion(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	currentVersion = func() string { return "dev" }
	cacheDirFunc = func() (string, error) { return t.TempDir(), nil }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name":"v2.0.0"}`)
	}))
	defer server.Close()

	doHTTPRequest = redirectToServer(server.URL)

	result, err := checkLatest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "dev" is not valid semver, so Available stays false.
	if result.Available {
		t.Error("expected Available=false for dev version")
	}
	if result.NewVersion != "2.0.0" {
		t.Errorf("expected NewVersion=2.0.0, got %q", result.NewVersion)
	}
}

// -------------------------------------------------------------------
// Cache tests
// -------------------------------------------------------------------

func TestCacheFilePath_Deterministic(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	fakeDir := "/home/user/.cache"
	cacheDirFunc = func() (string, error) { return fakeDir, nil }

	path1 := cacheFilePath()
	path2 := cacheFilePath()

	want := filepath.Join(fakeDir, cacheFile)
	if path1 != want {
		t.Errorf("cacheFilePath() = %q, want %q", path1, want)
	}
	if path1 != path2 {
		t.Errorf("cacheFilePath not deterministic: %q vs %q", path1, path2)
	}
}

func TestCacheFilePath_NoCacheDir(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	cacheDirFunc = func() (string, error) { return "", fmt.Errorf("no cache") }
	if got := cacheFilePath(); got != "" {
		t.Errorf("expected empty path when cache dir unavailable, got %q", got)
	}
}

func TestReadCache_Fresh(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	tmpDir := t.TempDir()
	cacheDirFunc = func() (string, error) { return tmpDir, nil }

	entry := cacheEntry{
		Available:  true,
		CheckedAt:  time.Now(),
		NewVersion: "2.0.0",
	}
	data, _ := json.Marshal(entry)
	path := filepath.Join(tmpDir, cacheFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	result := readCache()
	if result == nil {
		t.Fatal("expected cached result, got nil")
	}
	if !result.Available {
		t.Error("expected Available=true from cache")
	}
	if result.NewVersion != "2.0.0" {
		t.Errorf("expected NewVersion=2.0.0, got %q", result.NewVersion)
	}
}

func TestReadCache_Expired(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	tmpDir := t.TempDir()
	cacheDirFunc = func() (string, error) { return tmpDir, nil }

	entry := cacheEntry{
		Available:  true,
		CheckedAt:  time.Now().Add(-48 * time.Hour),
		NewVersion: "2.0.0",
	}
	data, _ := json.Marshal(entry)
	path := filepath.Join(tmpDir, cacheFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	result := readCache()
	if result != nil {
		t.Errorf("expected nil for expired cache, got %+v", result)
	}
}

func TestReadCache_CorruptJSON(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	tmpDir := t.TempDir()
	cacheDirFunc = func() (string, error) { return tmpDir, nil }

	path := filepath.Join(tmpDir, cacheFile)
	if err := os.WriteFile(path, []byte("{not valid}"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := readCache()
	if result != nil {
		t.Errorf("expected nil for corrupt cache, got %+v", result)
	}
}

func TestReadCache_NoFile(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	tmpDir := t.TempDir()
	cacheDirFunc = func() (string, error) { return tmpDir, nil }

	result := readCache()
	if result != nil {
		t.Errorf("expected nil when no cache file, got %+v", result)
	}
}

func TestReadCache_CacheDisabled(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	cacheDirFunc = func() (string, error) { return "", fmt.Errorf("no cache") }
	result := readCache()
	if result != nil {
		t.Errorf("expected nil when cache disabled, got %+v", result)
	}
}

func TestWriteAndReadCache(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	tmpDir := t.TempDir()
	cacheDirFunc = func() (string, error) { return tmpDir, nil }

	result := &CheckResult{Available: true, NewVersion: "3.0.0"}
	writeCache(result)

	read := readCache()
	if read == nil {
		t.Fatal("expected cached result after write, got nil")
	}
	if !read.Available {
		t.Error("expected Available=true")
	}
	if read.NewVersion != "3.0.0" {
		t.Errorf("expected NewVersion=3.0.0, got %q", read.NewVersion)
	}

	// Verify the file exists and is valid JSON.
	path := filepath.Join(tmpDir, cacheFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read cache file: %v", err)
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("cache file is not valid JSON: %v", err)
	}
}

func TestWriteCache_Disabled(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	cacheDirFunc = func() (string, error) { return "", fmt.Errorf("no cache") }
	// Should not panic.
	writeCache(&CheckResult{Available: true, NewVersion: "1.0.0"})
}

// -------------------------------------------------------------------
// PrintUpdateNotice tests
// -------------------------------------------------------------------

func TestPrintUpdateNotice_Available(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)
	currentVersion = func() string { return "1.0.0" }

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr
		_ = w.Close()
	}()

	result := &CheckResult{Available: true, NewVersion: "2.0.0"}
	PrintUpdateNotice(result)
	_ = w.Close()

	buf, _ := io.ReadAll(r)
	output := string(buf)

	if !strings.Contains(output, "1.0.0") {
		t.Errorf("expected output to contain current version, got: %q", output)
	}
	if !strings.Contains(output, "2.0.0") {
		t.Errorf("expected output to contain new version, got: %q", output)
	}
	if !strings.Contains(output, "dargstack update --self") {
		t.Errorf("expected update command in output, got: %q", output)
	}
}

func TestPrintUpdateNotice_NotAvailable(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr
		_ = w.Close()
	}()

	PrintUpdateNotice(&CheckResult{Available: false, NewVersion: "1.0.0"})
	_ = w.Close()

	buf, _ := io.ReadAll(r)
	if len(buf) > 0 {
		t.Errorf("expected no output when not available, got: %q", string(buf))
	}
}

func TestPrintUpdateNotice_Nil(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr
		_ = w.Close()
	}()

	PrintUpdateNotice(nil)
	_ = w.Close()

	buf, _ := io.ReadAll(r)
	if len(buf) > 0 {
		t.Errorf("expected no output for nil result, got: %q", string(buf))
	}
}

// -------------------------------------------------------------------
// BackgroundCheck / CollectBackgroundCheck tests
// -------------------------------------------------------------------

func TestBackgroundCheck_DevSkipped(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)
	currentVersion = func() string { return "dev" }

	BackgroundCheck()
	result := CollectBackgroundCheck()
	if result != nil {
		t.Errorf("expected nil for dev build, got %+v", result)
	}
}

func TestBackgroundCheck_FastResult(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	currentVersion = func() string { return "1.0.0" }
	cacheDirFunc = func() (string, error) { return t.TempDir(), nil }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name":"v2.0.0"}`)
	}))
	defer server.Close()

	doHTTPRequest = redirectToServer(server.URL)

	BackgroundCheck()
	result := CollectBackgroundCheck()
	if result == nil {
		t.Fatal("expected result from background check")
	}
	if !result.Available {
		t.Error("expected Available=true")
	}
	if result.NewVersion != "2.0.0" {
		t.Errorf("expected NewVersion=2.0.0, got %q", result.NewVersion)
	}
}

func TestBackgroundCheck_SlowResult(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	currentVersion = func() string { return "1.0.0" }
	cacheDirFunc = func() (string, error) { return t.TempDir(), nil }

	done := make(chan struct{})
	doHTTPRequest = func(req *http.Request) (*http.Response, error) {
		time.Sleep(200 * time.Millisecond)
		close(done)
		return nil, fmt.Errorf("timeout")
	}

	BackgroundCheck()
	result := CollectBackgroundCheck()
	if result != nil {
		t.Errorf("expected nil for slow check (timeout), got %+v", result)
	}

	// Wait for the background goroutine to finish before cleanup runs.
	<-done
}

func TestBackgroundCheck_OnlyRunsOnce(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)

	currentVersion = func() string { return "1.0.0" }
	cacheDirFunc = func() (string, error) { return t.TempDir(), nil }

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name":"v2.0.0"}`)
	}))
	defer server.Close()

	doHTTPRequest = redirectToServer(server.URL)

	BackgroundCheck()
	BackgroundCheck()
	BackgroundCheck()

	// Wait for the background goroutine to make the HTTP call before collecting.
	for i := 0; i < 50; i++ {
		if callCount.Load() == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	result := CollectBackgroundCheck()
	if result == nil {
		t.Fatal("expected result from background check")
	}

	if callCount.Load() != 1 {
		t.Errorf("expected exactly 1 HTTP call (sync.Once), got %d", callCount.Load())
	}
}

// -------------------------------------------------------------------
// SelfUpdate dev build test
// -------------------------------------------------------------------

func TestSelfUpdate_DevBuild(t *testing.T) {
	cleanup := testSetup(t)
	defer cleanup(t)
	currentVersion = func() string { return "dev" }

	err := SelfUpdate()
	if err == nil {
		t.Fatal("expected error for dev build")
	}
	if !strings.Contains(err.Error(), "development") {
		t.Errorf("expected 'development' in error, got: %v", err)
	}
}
