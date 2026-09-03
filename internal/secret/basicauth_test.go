package secret

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestFormatBasicAuthBcrypt(t *testing.T) {
	line, err := formatBasicAuth("admin", "hunter2")
	if err != nil {
		t.Fatalf("formatBasicAuth: %v", err)
	}

	user, hash, ok := strings.Cut(line, ":")
	if !ok {
		t.Fatalf("expected username:hash, got %q", line)
	}
	if user != "admin" {
		t.Errorf("username = %q, want admin", user)
	}
	if !strings.HasPrefix(hash, "$2y$") {
		t.Errorf("hash %q does not use the htpasswd $2y$ prefix", hash)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("hunter2")); err != nil {
		t.Errorf("hash does not verify against the password: %v", err)
	}
}

func TestFormatBasicAuthErrors(t *testing.T) {
	cases := []struct {
		name     string
		username string
		password string
	}{
		{"empty username", "", "pw"},
		{"username with colon", "ad:min", "pw"},
		{"empty password", "admin", ""},
		{"password past the bcrypt limit", "admin", strings.Repeat("a", MaxPasswordBytes+1)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := formatBasicAuth(c.username, c.password); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

func TestResolveBasicAuth(t *testing.T) {
	templates := map[string]Template{
		"admin-auth":     {Type: TypeBasicAuth, Username: "admin", Source: "admin-password"},
		"admin-password": {Type: TypeRandomString, Length: 24},
	}

	resolved, err := Resolve(templates, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	password := resolved["admin-password"]
	if password == "" {
		t.Fatal("source password was not generated")
	}

	user, hash, ok := strings.Cut(resolved["admin-auth"], ":")
	if !ok {
		t.Fatalf("expected username:hash, got %q", resolved["admin-auth"])
	}
	if user != "admin" {
		t.Errorf("username = %q, want admin", user)
	}
	// The hash must cover the generated plaintext, which proves the source secret was resolved first.
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		t.Errorf("hash does not match the generated source password: %v", err)
	}
}

func TestResolveBasicAuthUsernameFromSecret(t *testing.T) {
	templates := map[string]Template{
		"admin-auth":     {Type: TypeBasicAuth, Username: "{{secret:admin-user}}", Source: "admin-password"},
		"admin-password": {Type: TypeRandomString, Length: 16},
		"admin-user":     {Type: TypeWordlistWord},
	}

	resolved, err := Resolve(templates, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	user, _, _ := strings.Cut(resolved["admin-auth"], ":")
	if user != resolved["admin-user"] {
		t.Errorf("username = %q, want the resolved admin-user secret %q", user, resolved["admin-user"])
	}
}

func TestResolveBasicAuthSkipsUnsetSource(t *testing.T) {
	templates := map[string]Template{
		"admin-auth": {Type: TypeBasicAuth, Username: "admin", Source: "admin-password"},
		// A third-party password the user has not provided yet.
		"admin-password": {Type: TypeThirdParty},
	}
	values := map[string]string{"admin-password": ThirdPartyPlaceholder}

	resolved, err := ResolveAllowPlaceholders(templates, values)
	if err != nil {
		t.Fatalf("ResolveAllowPlaceholders: %v", err)
	}
	if v := resolved["admin-auth"]; v != "" {
		t.Errorf("expected no credential to be written for an unset password, got %q", v)
	}

	if _, err := Resolve(templates, values); err == nil {
		t.Error("expected Resolve to reject an unset source password")
	}
}

func TestNormalizeBasicAuthInference(t *testing.T) {
	tmpl := Template{Username: "admin", Source: "admin-password"}
	normalizeTemplate(&tmpl)

	if tmpl.Type != TypeBasicAuth {
		t.Errorf("type = %q, want %q", tmpl.Type, TypeBasicAuth)
	}
	if !IsAutoGeneratable(&tmpl) {
		t.Error("basic_auth should be auto-generatable")
	}
}
