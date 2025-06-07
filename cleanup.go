package main

import (
	"regexp"
	"strings"
)

// Cleanup for file and directory names generated from templates
var spacePattern = regexp.MustCompile(`[_\t\r\n]+`)

var multispacePattern = regexp.MustCompile(`\s+`)

var forbiddenCharPattern = regexp.MustCompile(`[<>:"'/\\|?*\x00-\x1F]`)

var trimPathPattern = regexp.MustCompile(`^[. ]+|[-. ]+$`)

var bracketPattern = regexp.MustCompile(`[\[\](){}]`)

func cleanPathSegment(pathSegment string) string {
	// Normalize Unicode (optional: requires a Unicode normalization lib)
	// Remove characters not safe for filenames
	// Keep letters, digits, some punctuation, spaces, dashes and underscores
	cleaned := forbiddenCharPattern.ReplaceAllString(pathSegment, "_")

	// Replace "special notifiers" in brackets like "(Explicit)" with safer delimiters
	cleaned = bracketPattern.ReplaceAllString(cleaned, " - ")

	// Shell-awkward characters
	cleaned = strings.ReplaceAll(cleaned, "`", "")    // Remove backticks
	cleaned = strings.ReplaceAll(cleaned, "&", "and") // Replace ampersand
	cleaned = strings.ReplaceAll(cleaned, "#", "No")  // Replace hash

	// Collapse multiple underscores/spaces
	cleaned = spacePattern.ReplaceAllString(cleaned, " ")
	cleaned = multispacePattern.ReplaceAllString(cleaned, " ")

	// Trim leading/trailing spaces and dots
	// Trimming leading dots avoids hidden file and path traversal
	// Trimming trailing dots avoids weird-looking file names
	cleaned = trimPathPattern.ReplaceAllString(cleaned, "")

	// Max 255 characters per segment
	if len(cleaned) > 255 {
		cleaned = cleaned[:255]
	}

	return cleaned
}

func cleanPath(path string) string {
	segments := strings.Split(path, "/")
	newSegments := make([]string, len(segments))
	for _, segment := range segments {
		cleanSegment := cleanPathSegment(segment)
		if cleanSegment != "" {
			newSegments = append(newSegments, cleanSegment)
		}
	}

	cleanedPath := strings.Join(newSegments, "/")

	// Avoid absolute paths, paths must always be relative and a file name
	cleanedPath = strings.Trim(cleanedPath, "/")
	return cleanedPath
}
