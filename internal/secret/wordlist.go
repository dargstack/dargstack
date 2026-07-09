package secret

import (
	_ "embed"
	"strings"
)

//go:embed data/english.txt
var englishWordlist string

var bip39Words = func() []string {
	lines := strings.Split(englishWordlist, "\n")
	var words []string
	for _, line := range lines {
		w := strings.TrimSpace(line)
		if w != "" {
			words = append(words, w)
		}
	}
	return words
}()
