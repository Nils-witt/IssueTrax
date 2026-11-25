package service_test

import (
	"errors"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/adapter/storage/memory"
	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
	"nilswitt.dev/issuetrax/internal/core/service"
)

func TestIssueService_Create(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name    string
		in      port.CreateIssueInput
		wantErr error
	}{
		{name: "valid", in: port.CreateIssueInput{Title: "  Fix login  ", Description: "details"}},
		{name: "empty title", in: port.CreateIssueInput{Title: "   "}, wantErr: domain.ErrInvalidIssue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := service.NewIssueService(memory.NewIssueRepository(),
				service.WithClock(func() time.Time { return now }),
				service.WithIDGenerator(func() domain.IssueID { return "id-1" }),
			)

			got, err := svc.Create(t.Context(), tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Create() error = %v, want %v", err, tt.wantErr)
			}

			if tt.wantErr != nil {
				return
			}

			if got.ID != "id-1" || got.Title != "Fix login" || got.Status != domain.StatusOpen || !got.CreatedAt.Equal(now) {
				t.Fatalf("Create() = %+v", got)
			}

			stored, err := svc.Get(t.Context(), got.ID)
			if err != nil {
				t.Fatalf("Get() error = %v", err)
			}

			if stored != got {
				t.Fatalf("Get() = %+v, want %+v", stored, got)
			}
		})
	}
}

func TestIssueService_GetNotFound(t *testing.T) {
	t.Parallel()

	svc := service.NewIssueService(memory.NewIssueRepository())

	_, err := svc.Get(t.Context(), "missing")
	if !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("Get() error = %v, want %v", err, domain.ErrIssueNotFound)
	}
}
