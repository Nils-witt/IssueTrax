package domain_test

import (
	"strings"
	"testing"
	"time"

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

func TestParseTagColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want domain.TagColor
		ok   bool
	}{
		{in: "#1d76db", want: "#1d76db", ok: true},
		{in: " #1D76DB ", want: "#1d76db", ok: true},
		{in: "1d76db", want: "1d76db"},
		{in: "#fff", want: "#fff"},
		{in: "#1d76dz", want: "#1d76dz"},
		{in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got, ok := domain.ParseTagColor(tt.in)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("ParseTagColor(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestNewTag_DefaultColor(t *testing.T) {
	t.Parallel()

	tag := domain.NewTag(domain.TagID{}, domain.ProjectID{}, "bug", time.Time{})

	if _, ok := domain.ParseTagColor(string(tag.Color)); !ok {
		t.Fatalf("NewTag() color = %q, want a #rrggbb color", tag.Color)
	}

	if again := domain.DefaultTagColor("bug"); again != tag.Color {
		t.Fatalf("DefaultTagColor(bug) = %q, want stable %q", again, tag.Color)
	}
}

func TestTag_Apply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		in         domain.TagChanges
		want       domain.Tag
		wantFields map[string]string
	}{
		{name: "nothing", in: domain.TagChanges{}, want: domain.Tag{Name: "old", Color: "#000000"}},
		{name: "normalizes name", in: domain.TagChanges{Name: new("Front End")}, want: domain.Tag{Name: "front-end", Color: "#000000"}},
		{name: "normalizes color", in: domain.TagChanges{Color: new("#ABCDEF")}, want: domain.Tag{Name: "old", Color: "#abcdef"}},
		{
			name: "both", in: domain.TagChanges{Name: new("new"), Color: new("#123456")},
			want: domain.Tag{Name: "new", Color: "#123456"},
		},
		{
			name: "empty name", in: domain.TagChanges{Name: new(" "), Color: new("#123456")},
			want: domain.Tag{Name: "old", Color: "#000000"}, wantFields: map[string]string{"name": domain.FieldRequired},
		},
		{
			name: "invalid name", in: domain.TagChanges{Name: new("#1")},
			want: domain.Tag{Name: "old", Color: "#000000"}, wantFields: map[string]string{"name": domain.FieldInvalid},
		},
		{
			name: "empty color", in: domain.TagChanges{Color: new("")},
			want: domain.Tag{Name: "old", Color: "#000000"}, wantFields: map[string]string{"color": domain.FieldRequired},
		},
		{
			name: "invalid both", in: domain.TagChanges{Name: new("#"), Color: new("red")},
			want:       domain.Tag{Name: "old", Color: "#000000"},
			wantFields: map[string]string{"name": domain.FieldInvalid, "color": domain.FieldInvalid},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tag := domain.Tag{Name: "old", Color: "#000000"}
			err := tag.Apply(tt.in)

			wantFields(t, err, tt.wantFields)

			if tag != tt.want {
				t.Fatalf("tag = %+v, want %+v", tag, tt.want)
			}
		})
	}
}
