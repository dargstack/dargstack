package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMergeFilesNoFiles(t *testing.T) {
	_, err := MergeFiles("")
	if err == nil {
		t.Fatal("expected error for zero files")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMergeFilesSingleFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "base.yaml")
	writeTestFile(t, f, "services:\n  web:\n    image: nginx\n")

	out, err := MergeFiles("", f)
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	services, ok := doc["services"].(map[string]interface{})
	if !ok {
		t.Fatal("expected services map")
	}
	if _, ok := services["web"]; !ok {
		t.Error("expected 'web' service in output")
	}
}

func TestMergeFilesOverride(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.yaml")
	overlay := filepath.Join(dir, "overlay.yaml")

	writeTestFile(t, base, "services:\n  web:\n    image: nginx:1.0\n    environment:\n      DEBUG: \"true\"\n")
	writeTestFile(t, overlay, "services:\n  web:\n    image: nginx:2.0\n")

	out, err := MergeFiles("", base, overlay)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(out), "nginx:2.0") {
		t.Error("expected overlay image nginx:2.0 to win")
	}
}

func TestMergeFilesPruneOperator(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.yaml")
	overlay := filepath.Join(dir, "overlay.yaml")

	writeTestFile(t, base, "services:\n  web:\n    image: nginx\n    environment:\n      DEBUG: \"true\"\n")
	writeTestFile(t, overlay, "services:\n  web:\n    environment:\n      DEBUG: (( prune ))\n")

	out, err := MergeFiles("", base, overlay)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(out), "DEBUG") {
		t.Error("expected DEBUG to be pruned from output")
	}
}

func TestMergeFileMissing(t *testing.T) {
	_, err := MergeFiles("", "/nonexistent/file.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestMergeFileInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "bad.yaml")
	writeTestFile(t, f, "{{not valid yaml")

	_, err := MergeFiles("", f)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadSingle(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "compose.yaml")
	content := "services:\n  web:\n    image: nginx\n"
	writeTestFile(t, f, content)

	out, err := LoadSingle("", f)
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	services, ok := doc["services"].(map[string]interface{})
	if !ok {
		t.Fatal("expected services map")
	}
	web, ok := services["web"].(map[string]interface{})
	if !ok {
		t.Fatal("expected web service")
	}
	if web["image"] != "nginx" {
		t.Errorf("expected image nginx, got %v", web["image"])
	}
}

func TestLoadSingleMissing(t *testing.T) {
	_, err := LoadSingle("", "/nonexistent/compose.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadSingleInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "bad.yaml")
	writeTestFile(t, f, "{{not valid")

	_, err := LoadSingle("", f)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestStripDevOnlyMarkers(t *testing.T) {
	input := `services:
  web:
    image: nginx
    deploy:
      labels:
        - "traefik.http.routers.web.rule=Host(` + "`web.app.localhost`" + `)"
        - "some.debug-label=true"  # dargstack:dev-only
    environment:
      DEBUG: "true"  # dargstack:dev-only
      NODE_ENV: "production"
`
	result := StripDevOnlyMarkers([]byte(input))
	output := string(result)

	if strings.Contains(output, "dargstack:dev-only") {
		t.Error("expected dev-only lines to be stripped")
	}
	if !strings.Contains(output, "traefik.http.routers") {
		t.Error("expected non-dev-only lines to remain")
	}
	if strings.Contains(output, "DEBUG") {
		t.Error("expected DEBUG dev-only line to be stripped")
	}
	if !strings.Contains(output, "NODE_ENV") {
		t.Error("expected NODE_ENV to remain")
	}
}

func TestStripDevOnlyMarkersNoMarkers(t *testing.T) {
	input := "services:\n  web:\n    image: nginx\n"
	result := StripDevOnlyMarkers([]byte(input))
	if !strings.Contains(string(result), "nginx") {
		t.Error("expected output to be unchanged when no markers present")
	}
}

func TestMergeEnvFiles(t *testing.T) {
	dir := t.TempDir()
	devEnv := filepath.Join(dir, "dev.env")
	prodEnv := filepath.Join(dir, "prod.env")

	writeTestFile(t, devEnv, "STACK_DOMAIN=app.localhost\nDEBUG=true\nSHARED=dev\n")
	writeTestFile(t, prodEnv, "STACK_DOMAIN=myapp.example.com\nSHARED=prod\n")

	result, err := MergeEnvFiles(devEnv, prodEnv)
	if err != nil {
		t.Fatal(err)
	}

	output := string(result)
	if !strings.Contains(output, "STACK_DOMAIN=myapp.example.com") {
		t.Error("expected production STACK_DOMAIN to override dev")
	}
	if !strings.Contains(output, "DEBUG=true") {
		t.Error("expected dev-only DEBUG to be preserved")
	}
	if !strings.Contains(output, "SHARED=prod") {
		t.Error("expected production SHARED to override dev")
	}
	if strings.Contains(output, "SHARED=dev") {
		t.Error("expected dev SHARED to be overridden")
	}
}

func TestMergeEnvFilesMissingDev(t *testing.T) {
	dir := t.TempDir()
	prodEnv := filepath.Join(dir, "prod.env")
	writeTestFile(t, prodEnv, "KEY=value\n")

	result, err := MergeEnvFiles(filepath.Join(dir, "nonexistent.env"), prodEnv)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), "KEY=value") {
		t.Error("expected prod env to still work when dev is missing")
	}
}

func TestRewriteProductionBindMountsShortSyntax(t *testing.T) {
	root := t.TempDir()
	devRoot := filepath.Join(root, "stack", "src", "development")
	prodRoot := filepath.Join(root, "stack", "src", "production")
	if err := os.MkdirAll(filepath.Join(devRoot, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(prodRoot, "api"), 0o755); err != nil {
		t.Fatal(err)
	}

	devCfg := filepath.Join(devRoot, "api", "config.yaml")
	prodCfg := filepath.Join(prodRoot, "api", "config.yaml")
	writeTestFile(t, devCfg, "dev")
	writeTestFile(t, prodCfg, "prod")

	input := []byte(fmt.Sprintf("services:\n  api:\n    volumes:\n      - %s:/etc/app/config.yaml\n", devCfg))
	out, err := RewriteProductionBindMounts(input, devRoot, prodRoot)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(out), prodCfg) {
		t.Fatalf("expected volume source to be rewritten to production path: %s", string(out))
	}
}

func TestRewriteProductionBindMountsKeepsNamedVolumes(t *testing.T) {
	root := t.TempDir()
	devRoot := filepath.Join(root, "stack", "src", "development")
	prodRoot := filepath.Join(root, "stack", "src", "production")

	input := []byte("services:\n  api:\n    volumes:\n      - pgdata:/var/lib/postgresql/data\n")
	out, err := RewriteProductionBindMounts(input, devRoot, prodRoot)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(out), "pgdata:/var/lib/postgresql/data") {
		t.Fatalf("expected named volume to remain unchanged: %s", string(out))
	}
}

func TestRewriteProductionBindMountsLongSyntax(t *testing.T) {
	root := t.TempDir()
	devRoot := filepath.Join(root, "stack", "src", "development")
	prodRoot := filepath.Join(root, "stack", "src", "production")
	if err := os.MkdirAll(filepath.Join(devRoot, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(prodRoot, "web"), 0o755); err != nil {
		t.Fatal(err)
	}

	devCfg := filepath.Join(devRoot, "web", "nginx.conf")
	prodCfg := filepath.Join(prodRoot, "web", "nginx.conf")
	writeTestFile(t, devCfg, "dev")
	writeTestFile(t, prodCfg, "prod")

	input := []byte(fmt.Sprintf("services:\n  web:\n    volumes:\n      - type: bind\n        source: %s\n        target: /etc/nginx/nginx.conf\n", devCfg))
	out, err := RewriteProductionBindMounts(input, devRoot, prodRoot)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(out), "source: "+prodCfg) {
		t.Fatalf("expected bind source to be rewritten to production path: %s", string(out))
	}
}

func TestStripProductionDevelopmentLabelsMapLabels(t *testing.T) {
	input := []byte(`services:
  api:
    deploy:
      labels:
        dargstack.development.build: ../../../../repository
        dargstack.development.debug: true
        dargstack.profiles: default
`)

	out, err := StripProductionDevelopmentLabels(input)
	if err != nil {
		t.Fatal(err)
	}

	output := string(out)
	if strings.Contains(output, "dargstack.development.build") {
		t.Fatal("expected dargstack.development.build to be removed")
	}
	if strings.Contains(output, "dargstack.development.debug") {
		t.Fatal("expected dargstack.development.* labels to be removed")
	}
	if !strings.Contains(output, "dargstack.profiles") {
		t.Fatal("expected dargstack.profiles to remain")
	}
}

func TestStripProductionDevelopmentLabelsListLabels(t *testing.T) {
	input := []byte(`services:
  api:
    deploy:
      labels:
        - dargstack.development.build=../../../../repository
        - dargstack.development.debug=true
        - dargstack.profiles=default
`)

	out, err := StripProductionDevelopmentLabels(input)
	if err != nil {
		t.Fatal(err)
	}

	output := string(out)
	if strings.Contains(output, "dargstack.development.build=") {
		t.Fatal("expected dargstack.development.build label entry to be removed")
	}
	if strings.Contains(output, "dargstack.development.debug=") {
		t.Fatal("expected dargstack.development.* label entries to be removed")
	}
	if !strings.Contains(output, "dargstack.profiles=default") {
		t.Fatal("expected dargstack.profiles label entry to remain")
	}
}

func TestMergeEnvFilesMissingProd(t *testing.T) {
	dir := t.TempDir()
	devEnv := filepath.Join(dir, "dev.env")
	writeTestFile(t, devEnv, "KEY=devvalue\n")

	result, err := MergeEnvFiles(devEnv, filepath.Join(dir, "nonexistent.env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), "KEY=devvalue") {
		t.Error("expected dev env to still work when prod is missing")
	}
}

func TestMergeEnvFilesCommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env")
	writeTestFile(t, envFile, "# comment\n\nKEY=value\n  \n# another\nFOO=bar\n")

	result, err := MergeEnvFiles(envFile, filepath.Join(dir, "nonexistent.env"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(result)
	if strings.Contains(output, "# comment") {
		t.Error("comments should not appear in merged output")
	}
	if !strings.Contains(output, "KEY=value") {
		t.Error("expected KEY=value in output")
	}
	if !strings.Contains(output, "FOO=bar") {
		t.Error("expected FOO=bar in output")
	}
}

func TestLoadEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	writeTestFile(t, envPath, "FOO=bar\nBAZ=\nQUX=hello world\n")

	env, err := LoadEnvFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if env["FOO"] != "bar" {
		t.Errorf("expected FOO=bar, got %s", env["FOO"])
	}
	if env["BAZ"] != "" {
		t.Errorf("expected BAZ=empty, got %s", env["BAZ"])
	}
	if env["QUX"] != "hello world" {
		t.Errorf("expected QUX=hello world, got %s", env["QUX"])
	}
}

func TestFindMissingEnvValues(t *testing.T) {
	env := map[string]string{"A": "1", "B": "", "C": "3", "D": ""}
	missing := FindMissingEnvValues(env)
	if len(missing) != 2 {
		t.Fatalf("expected 2 missing, got %d", len(missing))
	}
	if missing[0] != "B" || missing[1] != "D" {
		t.Errorf("expected [B D], got %v", missing)
	}
}

func TestWriteEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	env := map[string]string{"Z": "last", "A": "first", "M": "middle"}
	if err := WriteEnvFile(envPath, env); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	expected := "A=first\nM=middle\nZ=last\n"
	if string(data) != expected {
		t.Errorf("expected %q, got %q", expected, string(data))
	}
}

func TestResolveFilePathsSecrets(t *testing.T) {
	base := "/project/stack"
	secretDef := map[interface{}]interface{}{
		"file": "./secrets/my.secret",
	}
	doc := map[interface{}]interface{}{
		"secrets": map[interface{}]interface{}{
			"my": secretDef,
		},
	}

	resolveFilePaths(doc, base)

	got, _ := secretDef["file"].(string)
	want := filepath.Join(base, "secrets", "my.secret")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveFilePathsAbsoluteUnchanged(t *testing.T) {
	base := "/project/stack"
	secretDef := map[interface{}]interface{}{
		"file": "/absolute/path/my.secret",
	}
	doc := map[interface{}]interface{}{
		"secrets": map[interface{}]interface{}{
			"my": secretDef,
		},
	}

	resolveFilePaths(doc, base)

	got, _ := secretDef["file"].(string)
	if got != "/absolute/path/my.secret" {
		t.Errorf("expected absolute path unchanged, got %q", got)
	}
}

func TestResolveFilePathsBindMount(t *testing.T) {
	base := "/project"
	svcDef := map[interface{}]interface{}{
		"volumes": []interface{}{
			"./data:/var/data",
			"namedvol:/app",
		},
	}
	doc := map[interface{}]interface{}{
		"services": map[interface{}]interface{}{
			"app": svcDef,
		},
	}

	resolveFilePaths(doc, base)

	vols := svcDef["volumes"].([]interface{})
	if got := vols[0].(string); got != filepath.Join(base, "data")+":/var/data" {
		t.Errorf("expected relative bind mount resolved, got %q", got)
	}
	if got := vols[1].(string); got != "namedvol:/app" {
		t.Errorf("expected named volume unchanged, got %q", got)
	}
}
func TestSplitVolumeSpec(t *testing.T) {
	tests := []struct {
		host  string
		input string
		rest  string
	}{
		{"myvolume", "myvolume:/data", "/data"},
		{"/absolute", "/absolute:/container", "/container"},
		{"./relative", "./relative:/container", "/container"},
		{"named", "named:ro", "ro"},
		{"/src", "/src:/dst:ro", "/dst:ro"},
		{"/src", "/src:/dst:rw", "/dst:rw"},
		{"/src", "/src:/dst:ro,z", "/dst:ro,z"},
		{"named", "named:/data:rw", "/data:rw"},
		{"nocolon", "nocolon", ""},
		// Windows drive letters: treat C:\path as part of host, not as a split point
		{`C:\path`, `C:\path:/container`, "/container"},
		{"C:/path", "C:/path:/container", "/container"},
		{`C:\path`, `C:\path`, ""},
	}
	for _, tt := range tests {
		host, rest := splitVolumeSpec(tt.input)
		if host != tt.host || rest != tt.rest {
			t.Errorf("splitVolumeSpec(%q) = (%q, %q), want (%q, %q)",
				tt.input, host, rest, tt.host, tt.rest)
		}
	}
}

func TestExpandStackRootInSecrets(t *testing.T) {
	stackDir := "/project/stack"
	dir := t.TempDir()
	f := filepath.Join(dir, "compose.yaml")

	content := "secrets:\n  my-secret:\n    file: " + StackRootPrefix + "/artifacts/secrets/my.secret\n"
	writeTestFile(t, f, content)

	out, err := LoadSingle(stackDir, f)
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(stackDir, "artifacts", "secrets", "my.secret")
	if !strings.Contains(string(out), want) {
		t.Errorf("expected secret file path %q in output, got:\n%s", want, string(out))
	}
}

func TestExpandStackRootInBuildLabel(t *testing.T) {
	stackDir := "/project/stack"
	dir := t.TempDir()
	f := filepath.Join(dir, "compose.yaml")

	content := "services:\n  api:\n    image: api\n    deploy:\n      labels:\n        - dargstack.development.build=" + StackRootPrefix + "/../api\n"
	writeTestFile(t, f, content)

	out, err := LoadSingle(stackDir, f)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(out), stackDir) {
		t.Errorf("expected build label to contain stack dir %q, got:\n%s", stackDir, string(out))
	}
}

func TestExpandStackRootInVolume(t *testing.T) {
	stackDir := "/project/stack"
	dir := t.TempDir()
	f := filepath.Join(dir, "compose.yaml")

	content := "services:\n  api:\n    image: api\n    volumes:\n      - " + StackRootPrefix + "/../api:/srv/app\n"
	writeTestFile(t, f, content)

	out, err := LoadSingle(stackDir, f)
	if err != nil {
		t.Fatal(err)
	}

	// The ~~ prefix is expanded to absolute stack dir path.
	if !strings.Contains(string(out), stackDir) {
		t.Errorf("expected volume path to contain stack dir %q, got:\n%s", stackDir, string(out))
	}
	if !strings.Contains(string(out), ":/srv/app") {
		t.Errorf("expected volume target :/srv/app, got:\n%s", string(out))
	}
}

func TestExpandStackRootEmptySuffix(t *testing.T) {
	stackDir := "/project/stack"
	dir := t.TempDir()
	f := filepath.Join(dir, "compose.yaml")

	content := "services:\n  api:\n    image: api\n    environment:\n      STACK_DIR: \"" + StackRootPrefix + "\"\n"
	writeTestFile(t, f, content)

	out, err := LoadSingle(stackDir, f)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(out), stackDir) {
		t.Errorf("expected STACK_DIR to be expanded to %q, got:\n%s", stackDir, string(out))
	}
}

func TestExpandStackRootNoExpansion(t *testing.T) {
	stackDir := "/project/stack"
	dir := t.TempDir()
	f := filepath.Join(dir, "compose.yaml")

	writeTestFile(t, f, "services:\n  api:\n    image: nginx\n")

	out, err := LoadSingle(stackDir, f)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(out), "nginx") {
		t.Error("expected unchanged output when no stack root prefix present")
	}
}

func TestExpandStackRootWithMerge(t *testing.T) {
	stackDir := "/project/stack"
	dir := t.TempDir()
	base := filepath.Join(dir, "base.yaml")
	overlay := filepath.Join(dir, "overlay.yaml")

	baseContent := "secrets:\n  base-secret:\n    file: " + StackRootPrefix + "/artifacts/base.secret\n"
	overlayContent := "secrets:\n  overlay-secret:\n    file: " + StackRootPrefix + "/artifacts/overlay.secret\n"
	writeTestFile(t, base, baseContent)
	writeTestFile(t, overlay, overlayContent)

	out, err := MergeFiles(stackDir, base, overlay)
	if err != nil {
		t.Fatal(err)
	}

	baseWant := filepath.Join(stackDir, "artifacts", "base.secret")
	overlayWant := filepath.Join(stackDir, "artifacts", "overlay.secret")
	if !strings.Contains(string(out), baseWant) {
		t.Errorf("expected base secret path %q in output", baseWant)
	}
	if !strings.Contains(string(out), overlayWant) {
		t.Errorf("expected overlay secret path %q in output", overlayWant)
	}
}

func TestExpandParentDirInSecret(t *testing.T) {
	stackDir := "/project/stack"
	parentDir := "/project"
	dir := t.TempDir()
	f := filepath.Join(dir, "compose.yaml")

	content := "secrets:\n  app-secret:\n    file: " + ParentDirPrefix + "/shared/app.secret\n"
	writeTestFile(t, f, content)

	out, err := LoadSingle(stackDir, f)
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(parentDir, "shared", "app.secret")
	if !strings.Contains(string(out), want) {
		t.Errorf("expected secret path %q, got:\n%s", want, string(out))
	}
}

func TestExpandParentDirInVolume(t *testing.T) {
	stackDir := "/project/stack"
	parentDir := "/project"
	dir := t.TempDir()
	f := filepath.Join(dir, "compose.yaml")

	content := "services:\n  api:\n    image: api\n    volumes:\n      - " + ParentDirPrefix + "/api:/srv/app\n"
	writeTestFile(t, f, content)

	out, err := LoadSingle(stackDir, f)
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(parentDir, "api")
	if !strings.Contains(string(out), want) {
		t.Errorf("expected volume path to contain %q, got:\n%s", want, string(out))
	}
}

func TestExpandBothPrefixes(t *testing.T) {
	stackDir := "/project/stack"
	parentDir := "/project"
	dir := t.TempDir()
	f := filepath.Join(dir, "compose.yaml")

	content := "services:\n  api:\n    image: api\n    volumes:\n      - " + StackRootPrefix + "/local:/local\n      - " + ParentDirPrefix + "/shared:/shared\n"
	writeTestFile(t, f, content)

	out, err := LoadSingle(stackDir, f)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(out), stackDir) {
		t.Errorf("expected volume path to contain stack dir %q, got:\n%s", stackDir, string(out))
	}
	if !strings.Contains(string(out), parentDir) {
		t.Errorf("expected volume path to contain parent dir %q, got:\n%s", parentDir, string(out))
	}
}
