package prompt

import (
	"testing"
)

func TestConfirmNonInteractive(t *testing.T) {
	NonInteractive = true
	defer func() { NonInteractive = false }()

	tests := []struct {
		name       string
		title      string
		defaultVal bool
		want       bool
	}{
		{"default_true", "Proceed?", true, true},
		{"default_false", "Proceed?", false, false},
		{"empty_title_true", "", true, true},
		{"empty_title_false", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Confirm(tt.title, tt.defaultVal)
			if err != nil {
				t.Errorf("Confirm() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Confirm() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectNonInteractive(t *testing.T) {
	NonInteractive = true
	defer func() { NonInteractive = false }()

	tests := []struct {
		name    string
		title   string
		options []string
		want    string
	}{
		{"normal_options", "Choose:", []string{"a", "b", "c"}, ""},
		{"single_option", "Choose:", []string{"only"}, ""},
		{"empty_options", "Choose:", []string{}, ""},
		{"nil_options", "Choose:", nil, ""},
		{"empty_title", "", []string{"x"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Select(tt.title, tt.options)
			if err != nil {
				t.Errorf("Select() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Select() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMultiSelectNonInteractive(t *testing.T) {
	NonInteractive = true
	defer func() { NonInteractive = false }()

	tests := []struct {
		name    string
		title   string
		options []string
	}{
		{"normal_options", "Choose:", []string{"a", "b", "c"}},
		{"single_option", "Choose:", []string{"only"}},
		{"empty_options", "Choose:", []string{}},
		{"nil_options", "Choose:", nil},
		{"empty_title", "", []string{"x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MultiSelect(tt.title, tt.options)
			if err != nil {
				t.Errorf("MultiSelect() unexpected error: %v", err)
			}
			if got != nil {
				t.Errorf("MultiSelect() = %v, want nil", got)
			}
		})
	}
}

func TestInputNonInteractive(t *testing.T) {
	NonInteractive = true
	defer func() { NonInteractive = false }()

	tests := []struct {
		name       string
		title      string
		defaultVal string
		want       string
	}{
		{"with_default", "Name:", "alice", "alice"},
		{"empty_default", "Name:", "", ""},
		{"empty_title", "", "bob", "bob"},
		{"both_empty", "", "", ""},
		{"special_chars", "Key:", "a:b@c#d", "a:b@c#d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Input(tt.title, tt.defaultVal)
			if err != nil {
				t.Errorf("Input() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Input() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPasswordNonInteractive(t *testing.T) {
	NonInteractive = true
	defer func() { NonInteractive = false }()

	tests := []struct {
		name  string
		title string
	}{
		{"normal_title", "Password:"},
		{"empty_title", ""},
		{"long_title", "Enter your secret passphrase for authentication:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Password(tt.title)
			if err != nil {
				t.Errorf("Password() unexpected error: %v", err)
			}
			if got != "" {
				t.Errorf("Password() = %q, want empty string", got)
			}
		})
	}
}

func TestNonInteractiveReset(t *testing.T) {
	// Ensure NonInteractive is false by default and can be toggled
	if NonInteractive {
		t.Error("NonInteractive should be false by default")
	}

	NonInteractive = true
	if !NonInteractive {
		t.Error("NonInteractive should be true after setting")
	}

	NonInteractive = false
	if NonInteractive {
		t.Error("NonInteractive should be false after resetting")
	}
}
