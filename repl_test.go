package main

import (
	"testing"
)

func TestCleanInput(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "trims and splits a simple sentence",
			input:    "  hello  world  ",
			expected: []string{"hello", "world"},
		},
		{
			name:     "lowercases mixed-case input",
			input:    "Charmander Bulbasaur PIKACHU",
			expected: []string{"charmander", "bulbasaur", "pikachu"},
		},
		{
			name:     "collapses tabs and newlines as whitespace",
			input:    "catch\tpikachu\n  now",
			expected: []string{"catch", "pikachu", "now"},
		},
		{
			name:     "returns empty slice for empty input",
			input:    "",
			expected: []string{},
		},
		{
			name:     "returns empty slice for whitespace-only input",
			input:    "   \t\n  ",
			expected: []string{},
		},
		{
			name:     "handles a single word with no whitespace",
			input:    "Pokedex",
			expected: []string{"pokedex"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := cleanInput(c.input)
			if len(actual) != len(c.expected) {
				t.Errorf("expected %d words, got %d", len(c.expected), len(actual))
			}

			for i := range actual {
				word := actual[i]
				expectedWord := c.expected[i]

				if word != expectedWord {
					t.Errorf("expected %q, got %q", expectedWord, word)
				}
			}
		})
	}
}
