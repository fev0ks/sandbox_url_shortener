package shortcode

import (
	"testing"
)

func TestGenerate_Length(t *testing.T) {
	code, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if len(code) != Length {
		t.Errorf("expected length %d, got %d: %q", Length, len(code), code)
	}
}

func TestGenerate_ValidAlphabet(t *testing.T) {
	for i := 0; i < 1000; i++ {
		code, err := Generate()
		if err != nil {
			t.Fatalf("Generate() error on iteration %d: %v", i, err)
		}
		if !IsValid(code) {
			t.Errorf("iteration %d: generated code %q is not valid", i, code)
		}
	}
}

func TestGenerate_Uniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		code, err := Generate()
		if err != nil {
			t.Fatalf("Generate() error on iteration %d: %v", i, err)
		}
		if _, exists := seen[code]; exists {
			t.Errorf("duplicate code %q on iteration %d", code, i)
		}
		seen[code] = struct{}{}
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid code", "abc1234", true},
		{"too short", "abc123", false},
		{"too long", "abc12345", false},
		{"empty", "", false},
		{"invalid char 0", "0bc1234", false},
		{"invalid char O", "Obc1234", false},
		{"invalid char I", "Ibc1234", false},
		{"invalid char l", "lbc1234", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValid(tt.input); got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
