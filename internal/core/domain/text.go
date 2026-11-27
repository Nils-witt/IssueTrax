package domain

import (
	"strings"
	"unicode/utf8"
)

// Text limits.
const (
	// MaxDescriptionBytes bounds project and issue descriptions (Markdown).
	MaxDescriptionBytes = 256 << 10
	maxTitleRunes       = 200
	maxProjectNameRunes = 100
	// maxShortTextRunes bounds short optional texts such as a model name.
	maxShortTextRunes = 100
)

// checkRequiredText trims s and records required/too_long for field.
func checkRequiredText(f fieldErrors, field, s string, maxRunes int) string {
	s = strings.TrimSpace(s)

	switch {
	case s == "":
		f.add(field, FieldRequired)
	case utf8.RuneCountInString(s) > maxRunes:
		f.add(field, FieldTooLong)
	}

	return s
}

// checkDescription trims s and records too_long for field.
func checkDescription(f fieldErrors, field, s string) string {
	s = strings.TrimSpace(s)
	if len(s) > MaxDescriptionBytes {
		f.add(field, FieldTooLong)
	}

	return s
}

// checkOptionalText trims s and records too_long for field.
func checkOptionalText(f fieldErrors, field, s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > maxRunes {
		f.add(field, FieldTooLong)
	}

	return s
}

// dedupe returns s without duplicates, keeping first occurrences in order.
// It never returns nil.
func dedupe[T comparable](s []T) []T {
	seen := make(map[T]struct{}, len(s))
	out := make([]T, 0, len(s))

	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		out = append(out, v)
	}

	return out
}
