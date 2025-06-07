package main

import (
	"strings"
	"testing"
)

func TestCleanPathSegment(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"valid_name", "valid name"},
		{"invalid/name", "invalid name"},
		{"with spaces", "with spaces"},
		{" trailing_spaces ", "trailing spaces"},
		{"with   multiple   spaces", "with multiple spaces"},
		{"with_underscores_and_spaces", "with underscores and spaces"},
		{"\nwhitespace\tcharacters trailing\r\n", "whitespace characters trailing"},
		{"whitespace\t\r\ninside", "whitespace inside"},
		// Testing bracket handling at the end of segments
		{"(Explicit)", "- Explicit"},
		{"Copkiller (Explicit)", "Copkiller - Explicit"},
		{"[Album Title]", "- Album Title"},
		// Leading and traing slashes
		{".hidden_file", "hidden file"},
		{"../escaped/.hidden_file", "escaped .hidden file"},
		// special characters
		{"The Path separators: \\/", "The Path separators"},
		{"The shell busters: *\"'?\x01", "The shell busters"},
		// Special replacements
		{"Adam & the Ants", "Adam and the Ants"},
		{"Song #1", "Song No1"},
		{strings.Repeat("a", 300), strings.Repeat("a", 255)}, // Test for max length
	}
	for _, test := range tests {
		result := cleanPathSegment(test.input)
		if result != test.expected {
			t.Errorf("Expected '%s' but got '%s'", test.expected, result)
		}
	}
}

func TestCleanPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"valid/name/structure", "valid/name/structure"},
		{"invalid/name/with/forbidden<chars>", "invalid/name/with/forbidden chars"},
		{"with/multiple/slashes//and/spaces", "with/multiple/slashes/and/spaces"},
		{"/absolute/path", "absolute/path"},
		{"trailing/slash/", "trailing/slash"},
		{"hidden/.file", "hidden/file"},
		{"../path/traversal/../impossible/", "path/traversal/impossible"},
	}
	for _, test := range tests {
		result := cleanPath(test.input)
		if result != test.expected {
			t.Errorf("Expected '%s' but got '%s'", test.expected, result)
		}
	}
}
