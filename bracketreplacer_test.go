package main

import (
	"testing"
)

func TestBracketReplacer(t *testing.T) {
	tests := []struct {
		description  string
		configString string
		input        string
		expected     string
	}{
		{"simple bracket replacement", "(limited)", "Feelgood (limited edition superduper)", "Feelgood "},
		{"simple non-matching content replacement", "(limited)", "Feelgood (enhanced edition)", "Feelgood (enhanced edition)"},
		{"case-insensitive", "(limited)", "Feelgood (Limited Edition)", "Feelgood "},
		{"ignore non-specified brackets", "(limited)", "Feelgood (limited edition superduper) [digital] {fuzzy}", "Feelgood  [digital] {fuzzy}"},
		{"ignore non-specified brackets even when content matches", "(limited,fuzzy,digital)", "Feelgood (limited edition superduper) [digital] {fuzzy}", "Feelgood  [digital] {fuzzy}"},
		{"replace multiple brackets", "([{limited,fuzzy,digital}])", "Feelgood (limited edition superduper) [digital] {fuzzy}", "Feelgood   "},
		{"replace multiple brackets, with content left out", "([{limited,digital}])", "Feelgood (limited edition superduper) [digital] {fuzzy}", "Feelgood   {fuzzy}"},
		{"replace angle brackets", "([<limited,digital>])", "Feelgood <limited edition> [digital] {fuzzy}", "Feelgood   {fuzzy}"},
		{"empty brackets matches all content", "({})", "Feelgood (limited edition superduper) [digital] {fuzzy}", "Feelgood  [digital] "},
		{"star matches all content", "([*])", "Feelgood (limited edition superduper) [digital] {fuzzy}", "Feelgood   {fuzzy}"},
		{"ignores content outside brackets", "intro-ignored-text([*])fuzzy", "Feelgood (limited edition superduper) [digital] {fuzzy}", "Feelgood   {fuzzy}"},
		{"supports nested brackets, removes outer match", "([({limited,digital})])", "Feelgood (limited edition superduper [digital]) {fuzzy}", "Feelgood  {fuzzy}"},
		{"supports nested brackets, removes outer match", "([({digital})])", "Feelgood (limited edition superduper [digital]) {fuzzy}", "Feelgood  {fuzzy}"},
		{"supports nested brackets of the same type, removes outer match", "([({digital})])", "Feelgood (limited edition superduper (digital)) {fuzzy}", "Feelgood  {fuzzy}"},
		{"supports regular expression special characters in match", "([{.,+}])", "Feelgood (limited edition++) [1/2] {1.1}", "Feelgood  [1/2] "},
		{"supports other special characters in match", "([{/,#}])", "Feelgood (un/limited edition) [1.2] {#1}", "Feelgood  [1.2] "},

		// mismatched braces in input
		{"leaves input as-is when input has no closing bracket", "([({limited,digital})])", "Feelgood (limited edition [digital {fuzzy}", "Feelgood (limited edition [digital {fuzzy}"},
		{"replaces outer bracket match even if inner bracket is mismatched", "([({limited,digital})])", "Feelgood (limited edition [digital) {fuzzy}", "Feelgood  {fuzzy}"},
		{"leaves input as-is when outer bracket is mismatched", "([({limited,digital})])", "Feelgood (limited edition [digital] {fuzzy}", "Feelgood (limited edition [digital] {fuzzy}"},
		{"leaves input as-is when input has no opening bracket", "([({limited,digital})])", "Feelgood limited edition digital]) {fuzzy}", "Feelgood limited edition digital]) {fuzzy}"},

		// "weird" cases
		{"empty config means no replacement", "", "Feelgood (limited edition superduper)", "Feelgood (limited edition superduper)"},
		{"no brackets in input", "({[fuzzy,digital]})", "Feelgood, no specifiers", "Feelgood, no specifiers"},
		{"supports additional closing brackets", "([{limited,digital})])", "Feelgood (limited edition superduper) [digital] {fuzzy}", "Feelgood   {fuzzy}"},
		{"supports missing closing brackets", "([({limited,digital", "Feelgood (limited edition superduper) [digital] {fuzzy}", "Feelgood   {fuzzy}"},
		{"supports multiple opening brackets of the same type", "({[({limited,digital})])", "Feelgood (limited edition superduper) [digital] {fuzzy}", "Feelgood   {fuzzy}"},
		{"leaves input as-is when config has no brackets", "limited,digital", "Feelgood (limited edition) [digital]", "Feelgood (limited edition) [digital]"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			br := NewBracketReplacer(test.configString)
			actual := br.Replace(test.input, "")
			if actual != test.expected {
				t.Errorf("BracketReplacer(%q).Replace(%q) = %q; want %q", test.configString, test.input, actual, test.expected)
			}
		})
	}
}

func TestReplaceInBrackets(t *testing.T) {
	input := "All that she wants (Extended Version)"
	expected := "All that she wants XXL"
	conf := "(extended)"
	replacement := "XXL"
	actual := ReplaceInBrackets(conf, replacement, input)
	if actual != expected {
		t.Errorf("ReplaceInBrackets(%q, %q, %q) = %q; want %q", conf, replacement, input, expected, actual)
	}
}

func TestRemoveBrackets(t *testing.T) {
	input := "All that she wants (Extended Version)"
	expected := "All that she wants "
	conf := "(extended)"
	actual := RemoveBrackets(conf, input)
	if actual != expected {
		t.Errorf("ReplaceInBrackets(%q, %q) = %q; want %q", conf, input, expected, actual)
	}
}
