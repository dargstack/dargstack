package prompt

import (
	"os"

	"charm.land/huh/v2"
)

// NonInteractive disables all prompts when set to true.
// Calls to Confirm, Select, MultiSelect, Input, and Password return their default values immediately.
var NonInteractive bool

// Confirm asks a yes/no question.
// Returns the default value when NonInteractive is true or when the prompt cannot be completed (e.g., non-TTY terminal, user abort).
// Never returns an error for prompt failures: callers can treat the return value as the definitive answer.
func Confirm(title string, defaultVal bool) (bool, error) {
	if NonInteractive {
		return defaultVal, nil
	}
	var result bool
	err := huh.NewConfirm().
		Title(title).
		Value(&result).
		Affirmative("Yes").
		Negative("No").
		Run()
	if err != nil {
		// Degrade gracefully: non-TTY, user abort, or terminal error all fall back to the default rather than surfacing a prompt error.
		return defaultVal, nil
	}
	return result, nil
}

// Select presents a list of options and returns the selected value.
// Returns an empty string when NonInteractive is true.
func Select(title string, options []string) (string, error) {
	if NonInteractive {
		return "", nil
	}
	opts := make([]huh.Option[string], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, o)
	}

	var result string
	err := huh.NewSelect[string]().
		Title(title).
		Options(opts...).
		Value(&result).
		Run()
	if err != nil {
		return "", err
	}
	return result, nil
}

// MultiSelect presents a list of options and returns the selected values.
// Returns nil when NonInteractive is true.
func MultiSelect(title string, options []string) ([]string, error) {
	if NonInteractive {
		return nil, nil
	}
	opts := make([]huh.Option[string], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, o)
	}

	var result []string
	err := huh.NewMultiSelect[string]().
		Title(title).
		Options(opts...).
		Value(&result).
		Run()
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Input asks for a text value with an optional default.
// Returns defaultVal when NonInteractive is true.
func Input(title, defaultVal string) (string, error) {
	if NonInteractive {
		return defaultVal, nil
	}
	var result string
	input := huh.NewInput().
		Title(title).
		Value(&result).
		Placeholder(defaultVal)
	if err := input.Run(); err != nil {
		return defaultVal, err
	}
	if result == "" {
		return defaultVal, nil
	}
	return result, nil
}

// Password asks for a secret text value (masked input).
// Returns an empty string when NonInteractive is true.
func Password(title string) (string, error) {
	if NonInteractive {
		return "", nil
	}
	var result string
	input := huh.NewInput().
		Title(title).
		Value(&result).
		EchoMode(huh.EchoModePassword)
	if err := input.Run(); err != nil {
		return "", err
	}
	return result, nil
}

// Interactive reports whether a prompt can actually be answered right now.
// It is false with --no-interaction and whenever stdin or stdout is not a terminal, which covers pipes, CI runners, and cron jobs.
func Interactive() bool {
	if NonInteractive {
		return false
	}
	return isTerminal(os.Stdin) && isTerminal(os.Stdout)
}

// isTerminal reports whether f is a character device rather than a pipe, a file, or a closed descriptor.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
