package secret

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// MaxPasswordBytes is the longest password bcrypt accepts; x/crypto returns an error past it rather than silently truncating.
const MaxPasswordBytes = 72

// formatBasicAuth renders a single htpasswd line (username:hash) from a plaintext password.
// The result is what Traefik's basicauth middleware and nginx's auth_basic_user_file expect.
func formatBasicAuth(username, password string) (string, error) {
	if username == "" {
		return "", fmt.Errorf("username is empty")
	}
	if strings.ContainsAny(username, ":\n") {
		return "", fmt.Errorf("username %q must not contain %q or a newline", username, ":")
	}
	if password == "" {
		return "", fmt.Errorf("password is empty")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("generate bcrypt hash: %w", err)
	}

	// x/crypto emits the $2a$ prefix while htpasswd -B emits $2y$ for the identical format.
	// Rewrite it so the output matches htpasswd byte for byte, which keeps hashes readable by crypt(3) implementations that only accept $2y$.
	return username + ":$2y$" + strings.TrimPrefix(string(hash), "$2a$"), nil
}
