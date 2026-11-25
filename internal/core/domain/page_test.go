package domain_test

import (
	"testing"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestNewPageRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		limit      int
		want       int
		wantFields map[string]string
	}{
		{name: "default", limit: 0, want: domain.DefaultPageSize},
		{name: "one", limit: 1, want: 1},
		{name: "max", limit: domain.MaxPageSize, want: domain.MaxPageSize},
		{name: "above max", limit: domain.MaxPageSize + 1, wantFields: map[string]string{"limit": domain.FieldOutOfRange}},
		{name: "negative", limit: -1, wantFields: map[string]string{"limit": domain.FieldOutOfRange}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.NewPageRequest(tt.limit, "c")
			wantFields(t, err, tt.wantFields)

			if tt.wantFields == nil && (got.Limit != tt.want || got.Cursor != "c") {
				t.Fatalf("NewPageRequest() = %+v", got)
			}
		})
	}
}

func TestDecodeCursor(t *testing.T) {
	t.Parallel()

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()

		in := domain.ProjectCursor{Name: "issue trax", ID: "id"}

		got, ok, err := domain.DecodeCursor[domain.ProjectCursor](domain.EncodeCursor(in))
		if err != nil || !ok || got != in {
			t.Fatalf("DecodeCursor() = %+v, %v, %v; want %+v", got, ok, err, in)
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		_, ok, err := domain.DecodeCursor[domain.IssueCursor]("")
		if err != nil || ok {
			t.Fatalf("DecodeCursor(\"\") = %v, %v; want false, nil", ok, err)
		}
	})

	t.Run("malformed", func(t *testing.T) {
		t.Parallel()

		_, _, err := domain.DecodeCursor[domain.IssueCursor]("%%%")
		wantFields(t, err, map[string]string{"cursor": domain.FieldInvalid})
	})
}
