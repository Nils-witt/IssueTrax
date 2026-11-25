package domain_test

import (
	"strings"
	"testing"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestNormalizeTagName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want domain.TagName
		ok   bool
	}{
		{in: "Needs Review", want: "needs-review", ok: true},
		{in: "  bug ", want: "bug", ok: true},
		{in: "a_b  c", want: "a-b-c", ok: true},
		{in: "v2", want: "v2", ok: true},
		{in: strings.Repeat("a", 30), want: domain.TagName(strings.Repeat("a", 30)), ok: true},
		{in: strings.Repeat("a", 31), want: domain.TagName(strings.Repeat("a", 31))},
		{in: "-x", want: "-x"},
		{in: "über", want: "über"},
		{in: "a,b", want: "a,b"},
		{in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got, ok := domain.NormalizeTagName(tt.in)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("NormalizeTagName(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestTag_Rename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		in         string
		want       domain.TagName
		wantFields map[string]string
	}{
		{name: "normalizes", in: "Front End", want: "front-end"},
		{name: "empty", in: " ", want: "old", wantFields: map[string]string{"name": domain.FieldRequired}},
		{name: "invalid", in: "#1", want: "old", wantFields: map[string]string{"name": domain.FieldInvalid}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tag := domain.Tag{Name: "old"}
			err := tag.Rename(tt.in)

			wantFields(t, err, tt.wantFields)

			if tag.Name != tt.want {
				t.Fatalf("Name = %q, want %q", tag.Name, tt.want)
			}
		})
	}
}
