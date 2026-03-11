package prompt

import (
	"github.com/charmbracelet/huh"
)

// Confirm asks a yes/no question. Returns true for yes.
// In non-interactive mode, returns the default value.
func Confirm(title string, defaultVal bool) (bool, error) {
	var result bool
	err := huh.NewConfirm().
		Title(title).
		Value(&result).
		Affirmative("Yes").
		Negative("No").
		Run()
	if err != nil {
		return defaultVal, err
	}
	return result, nil
}

// Select presents a list of options and returns the selected value.
func Select(title string, options []string) (string, error) {
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
func MultiSelect(title string, options []string) ([]string, error) {
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
func Input(title, defaultVal string) (string, error) {
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
func Password(title string) (string, error) {
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
