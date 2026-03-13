package secret

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractTemplates(t *testing.T) {
	composeYAML := `services:
  postgres:
    image: postgres:16
x-dargstack:
  secrets:
    postgres-password:
      length: 16
      special_characters: true
    api-db-url:
      template: "postgresql://user:{{postgres-password}}@postgres:5432/db"
`

	templates, err := ExtractTemplates([]byte(composeYAML))
	if err != nil {
		t.Fatal(err)
	}

	if len(templates) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(templates))
	}

	pgTmpl := templates["postgres-password"]
	if pgTmpl.SpecialCharacters == nil || !*pgTmpl.SpecialCharacters {
		t.Errorf("expected special_characters=true")
	}
	if pgTmpl.Length != 16 {
		t.Errorf("expected length=16, got %d", pgTmpl.Length)
	}
	if pgTmpl.Type != TypeRandomString {
		t.Errorf("expected type=%s, got %s", TypeRandomString, pgTmpl.Type)
	}

	dbTmpl := templates["api-db-url"]
	if dbTmpl.Template == "" {
		t.Error("expected template to be set")
	}
}

func TestExtractTemplatesNoExtension(t *testing.T) {
	composeYAML := `services:
  api:
    image: api:latest
`
	templates, err := ExtractTemplates([]byte(composeYAML))
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 0 {
		t.Errorf("expected 0 templates, got %d", len(templates))
	}
}

func TestTopologicalSort(t *testing.T) {
	templates := map[string]Template{
		"db_password": {Length: 32},
		"db_user":     {},
		"db_url":      {Template: "postgresql://{{db_user}}:{{db_password}}@db:5432/app"},
	}

	sorted, err := TopologicalSort(templates)
	if err != nil {
		t.Fatal(err)
	}

	if len(sorted) != 3 {
		t.Fatalf("expected 3 items, got %d", len(sorted))
	}

	posMap := make(map[string]int)
	for i, name := range sorted {
		posMap[name] = i
	}

	if posMap["db_url"] < posMap["db_password"] {
		t.Error("db_url should come after db_password")
	}
	if posMap["db_url"] < posMap["db_user"] {
		t.Error("db_url should come after db_user")
	}
}

func TestTopologicalSortCircular(t *testing.T) {
	templates := map[string]Template{
		"a": {Template: "{{b}}"},
		"b": {Template: "{{a}}"},
	}

	_, err := TopologicalSort(templates)
	if err == nil {
		t.Error("expected error for circular dependency")
	}
}

func TestTopologicalSortUnknownRef(t *testing.T) {
	templates := map[string]Template{
		"a": {Template: "{{unknown}}"},
	}

	_, err := TopologicalSort(templates)
	if err == nil {
		t.Fatal("expected error for unknown reference")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("expected 'unknown' in error, got: %s", err.Error())
	}
	if strings.Contains(err.Error(), "circular") {
		t.Errorf("should not say 'circular' for missing reference, got: %s", err.Error())
	}
}

func TestResolve(t *testing.T) {
	templates := map[string]Template{
		"db_password": {Length: 16},
		"db_user":     {},
		"db_url":      {Template: "postgresql://{{db_user}}:{{db_password}}@db:5432/app"},
	}

	values := map[string]string{"db_user": "postgres"}

	resolved, err := Resolve(templates, values)
	if err != nil {
		t.Fatal(err)
	}

	if resolved["db_user"] != "postgres" {
		t.Errorf("expected db_user=postgres, got %q", resolved["db_user"])
	}
	if len(resolved["db_password"]) != 16 {
		t.Errorf("expected 16-char password, got %d chars", len(resolved["db_password"]))
	}
	if resolved["db_url"] == "" {
		t.Error("expected db_url to be resolved")
	}
	expected := "postgresql://postgres:" + resolved["db_password"] + "@db:5432/app"
	if resolved["db_url"] != expected {
		t.Errorf("expected %q, got %q", expected, resolved["db_url"])
	}
}

func TestResolvePreservesExisting(t *testing.T) {
	templates := map[string]Template{
		"password": {Length: 32},
	}

	values := map[string]string{"password": "existing_value"}

	resolved, err := Resolve(templates, values)
	if err != nil {
		t.Fatal(err)
	}

	if resolved["password"] != "existing_value" {
		t.Errorf("expected existing value to be preserved, got %q", resolved["password"])
	}
}

func TestWriteAndReadSecrets(t *testing.T) {
	dir := t.TempDir()
	values := map[string]string{
		"db-password":       "s3cret",
		"api-db-connection": "pg://localhost",
	}

	paths := map[string]string{
		"db-password":       filepath.Join(dir, "db", "password.secret"),
		"api-db-connection": filepath.Join(dir, "api", "db_connection.secret"),
	}

	if err := WriteSecrets(paths, values); err != nil {
		t.Fatal(err)
	}

	read := ReadSecretValues(paths)

	if read["db-password"] != "s3cret" {
		t.Errorf("expected s3cret, got %q", read["db-password"])
	}
	if read["api-db-connection"] != "pg://localhost" {
		t.Errorf("expected pg://localhost, got %q", read["api-db-connection"])
	}

	// Path not in map should not appear
	extraPaths := map[string]string{
		"db-password": filepath.Join(dir, "db", "password.secret"),
		"nonexistent": filepath.Join(dir, "nope"),
	}
	read2 := ReadSecretValues(extraPaths)
	if _, ok := read2["nonexistent"]; ok {
		t.Error("expected nonexistent to not be in read values")
	}
}

func TestUnresolvedSecrets(t *testing.T) {
	templates := map[string]Template{
		"auto_gen":     {Length: 16},
		"needs_prompt": {},
		"has_template": {Template: "{{auto_gen}}-suffix"},
	}

	values := map[string]string{}
	unresolved := UnresolvedSecrets(templates, values)

	if len(unresolved) != 1 {
		t.Fatalf("expected 1 unresolved, got %d: %v", len(unresolved), unresolved)
	}
	if unresolved[0] != "needs_prompt" {
		t.Errorf("expected needs_prompt, got %s", unresolved[0])
	}
}

func TestExtractTemplateRefs(t *testing.T) {
	refs := extractTemplateRefs("postgresql://{{user}}:{{pass}}@{{host}}:5432/{{db}}")
	if len(refs) != 4 {
		t.Fatalf("expected 4 refs, got %d", len(refs))
	}
	expected := []string{"user", "pass", "host", "db"}
	for i, ref := range refs {
		if ref != expected[i] {
			t.Errorf("ref[%d] = %q, want %q", i, ref, expected[i])
		}
	}
}

func TestResolveTemplate(t *testing.T) {
	result, err := resolveTemplate("{{a}}-{{b}}", map[string]string{"a": "hello", "b": "world"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello-world" {
		t.Errorf("expected hello-world, got %q", result)
	}
}

func TestResolveTemplateSpecialTokens(t *testing.T) {
	result, err := resolveTemplate("{{random:8:false}}-{{word}}-{{secret:name}}", map[string]string{"name": "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(result, "-alice") {
		t.Fatalf("expected token substitution with secret:name, got %q", result)
	}
}

func TestRandomStringDefaults(t *testing.T) {
	// When type=random_string is set without length/special_characters,
	// normalizeTemplate should default length=32 and special_characters=true.
	tmpl := Template{Type: TypeRandomString}
	normalizeTemplate(&tmpl)
	if tmpl.Length != 32 {
		t.Errorf("expected default length 32, got %d", tmpl.Length)
	}
	if tmpl.SpecialCharacters == nil || !*tmpl.SpecialCharacters {
		t.Error("expected default special_characters=true")
	}
}

func TestRandomStringSpecialCharactersOptOut(t *testing.T) {
	f := false
	tmpl := Template{Length: 16, SpecialCharacters: &f}
	normalizeTemplate(&tmpl)
	if tmpl.SpecialCharacters == nil || *tmpl.SpecialCharacters {
		t.Error("expected special_characters=false when explicitly set")
	}
}

func TestGenerateRandom(t *testing.T) {
	val, err := generateRandom(24, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(val) != 24 {
		t.Errorf("expected length 24, got %d", len(val))
	}

	val2, _ := generateRandom(24, false)
	if val == val2 {
		t.Error("expected different random values")
	}
}

func TestRewriteSecretFilePaths(t *testing.T) {
	composeYAML := `secrets:
  postgres-password:
    file: ./services/postgres/secrets/postgres.password
  api-key:
    file: ./services/api/secrets/api.key
`
	secretsDir := "/stack/artifacts/secrets"
	result, err := RewriteSecretFilePaths([]byte(composeYAML), secretsDir)
	if err != nil {
		t.Fatal(err)
	}

	paths := ExtractSecretPaths(result)
	wantPostgres := filepath.Join(secretsDir, "postgres-password")
	wantAPI := filepath.Join(secretsDir, "api-key")
	if got := paths["postgres-password"]; got != wantPostgres {
		t.Errorf("postgres-password path = %q, want %q", got, wantPostgres)
	}
	if got := paths["api-key"]; got != wantAPI {
		t.Errorf("api-key path = %q, want %q", got, wantAPI)
	}
}

func TestNormalizeTemplateAliases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"random", TypeRandomString},
		{"RANDOM_STRING", TypeRandomString},
		{" Random_String ", TypeRandomString},
		{"thirdparty", TypeThirdParty},
		{"insecure_default", TypeInsecureValue},
		{"default", TypeInsecureValue},
		{"private_key", TypePrivateKey},
		{"privatekey", TypePrivateKey},
		{"wordlist", TypeWord},
		{"wordlist_word", TypeWord},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tmpl := Template{Type: tt.input}
			normalizeTemplate(&tmpl)
			if tmpl.Type != tt.expected {
				t.Errorf("type %q: expected %q, got %q", tt.input, tt.expected, tmpl.Type)
			}
		})
	}
}

func TestTemplateDependency(t *testing.T) {
	tests := []struct {
		token   string
		wantDep string
		wantOk  bool
	}{
		{"secret:my-password", "my-password", true},
		{"secret: my-password ", "my-password", true},
		{"my-other-secret", "my-other-secret", true},
		{"word", "", false},
		{"private_key", "", false},
		{"random:16:true", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			dep, ok := templateDependency(tt.token)
			if ok != tt.wantOk {
				t.Errorf("token %q: ok=%v, want %v", tt.token, ok, tt.wantOk)
			}
			if dep != tt.wantDep {
				t.Errorf("token %q: dep=%q, want %q", tt.token, dep, tt.wantDep)
			}
		})
	}
}

func TestResolveInsecureDefault(t *testing.T) {
	templates := map[string]Template{
		"db_password": {Type: TypeInsecureValue, InsecureDefault: "changeme"},
	}
	resolved, err := Resolve(templates, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resolved["db_password"] != "changeme" {
		t.Errorf("expected changeme, got %q", resolved["db_password"])
	}
}

func TestResolveThirdPartySkipped(t *testing.T) {
	templates := map[string]Template{
		"api-key": {Type: TypeThirdParty},
	}
	resolved, err := Resolve(templates, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := resolved["api-key"]; ok && v != "" {
		t.Errorf("expected third_party secret to remain unset, got %q", v)
	}
}

func TestResolveWord(t *testing.T) {
	templates := map[string]Template{
		"passphrase": {Type: TypeWord},
	}
	resolved, err := Resolve(templates, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resolved["passphrase"] == "" {
		t.Error("expected non-empty word value")
	}
}

func TestResolvePrivateKey(t *testing.T) {
	templates := map[string]Template{
		"ssh.key": {Type: TypePrivateKey},
	}
	resolved, err := Resolve(templates, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resolved["ssh.key"], "PRIVATE KEY") {
		t.Errorf("expected PEM private key, got %q", resolved["ssh.key"])
	}
}

func TestSpecialCharsEnabled(t *testing.T) {
	trueVal := true
	falseVal := false

	if !specialCharsEnabled(&Template{}) {
		t.Error("expected true when SpecialCharacters is nil")
	}
	if !specialCharsEnabled(&Template{SpecialCharacters: &trueVal}) {
		t.Error("expected true when SpecialCharacters is true")
	}
	if specialCharsEnabled(&Template{SpecialCharacters: &falseVal}) {
		t.Error("expected false when SpecialCharacters is false")
	}
}
