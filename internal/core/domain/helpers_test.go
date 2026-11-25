package domain_test

import (
	"errors"
	"maps"
	"testing"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// wantFields fails unless err is a validation error with exactly fields.
func wantFields(t *testing.T, err error, fields map[string]string) {
	t.Helper()

	if len(fields) == 0 {
		if err != nil {
			t.Fatalf("error = %v, want nil", err)
		}

		return
	}

	de, ok := errors.AsType[*domain.Error](err)
	if !ok || !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("error = %v, want validation error %v", err, fields)
	}

	if !maps.Equal(de.Fields, fields) {
		t.Fatalf("fields = %v, want %v", de.Fields, fields)
	}
}
