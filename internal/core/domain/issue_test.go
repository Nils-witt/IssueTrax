package domain_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestIssueStatus_Valid(t *testing.T) {
	t.Parallel()

	for _, s := range domain.IssueStatuses() {
		if !s.Valid() {
			t.Errorf("%q.Valid() = false", s)
		}
	}

	for _, s := range []domain.IssueStatus{"", "OPEN", "done"} {
		if s.Valid() {
			t.Errorf("%q.Valid() = true", s)
		}
	}
}

func TestIssue_Key(t *testing.T) {
	t.Parallel()

	if got := (domain.Issue{Number: 42}).Key("TRAX"); got != "TRAX-42" {
		t.Fatalf("Key() = %q, want TRAX-42", got)
	}
}

func TestNewIssue(t *testing.T) {
	t.Parallel()

	tag := domain.TagID(uuid.New())

	tests := []struct {
		name       string
		params     domain.NewIssueParams
		wantStatus domain.IssueStatus
		wantFields map[string]string
	}{
		{name: "defaults to open", params: domain.NewIssueParams{Title: " Bug "}, wantStatus: domain.StatusOpen},
		{name: "explicit draft", params: domain.NewIssueParams{Title: "Bug", Status: domain.StatusDraft}, wantStatus: domain.StatusDraft},
		{name: "title at limit", params: domain.NewIssueParams{Title: strings.Repeat("ü", 200)}, wantStatus: domain.StatusOpen},
		{
			name:       "dedupes tags",
			params:     domain.NewIssueParams{Title: "Bug", Tags: []domain.TagID{tag, tag}},
			wantStatus: domain.StatusOpen,
		},
		{
			name:       "missing title and bad status",
			params:     domain.NewIssueParams{Title: "  ", Status: "done"},
			wantFields: map[string]string{"title": domain.FieldRequired, "status": domain.FieldInvalid},
		},
		{
			name:       "title too long",
			params:     domain.NewIssueParams{Title: strings.Repeat("a", 201)},
			wantFields: map[string]string{"title": domain.FieldTooLong},
		},
		{
			name:       "description too long",
			params:     domain.NewIssueParams{Title: "a", Description: strings.Repeat("a", domain.MaxDescriptionBytes+1)},
			wantFields: map[string]string{"description": domain.FieldTooLong},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.params.Now = testNow

			got, err := domain.NewIssue(tt.params)
			wantFields(t, err, tt.wantFields)

			if tt.wantFields != nil {
				return
			}

			if got.Status != tt.wantStatus || got.Title != strings.TrimSpace(tt.params.Title) || !got.CreatedAt.Equal(testNow) {
				t.Fatalf("NewIssue() = %+v", got)
			}

			if len(got.Tags) != len(slices.Compact(slices.Clone(tt.params.Tags))) {
				t.Fatalf("Tags = %v, want deduplicated %v", got.Tags, tt.params.Tags)
			}
		})
	}
}

func TestIssue_Apply(t *testing.T) {
	t.Parallel()

	later := testNow.Add(time.Hour)
	tag := domain.TagID(uuid.New())
	user := domain.UserID(uuid.New())

	tests := []struct {
		name       string
		status     domain.IssueStatus
		changes    domain.IssueChanges
		wantErr    error
		wantFields map[string]string
		check      func(t *testing.T, i domain.Issue)
	}{
		{
			name:    "edit open issue",
			status:  domain.StatusOpen,
			changes: domain.IssueChanges{Title: new("New"), Tags: &[]domain.TagID{tag, tag}, Assignees: &[]domain.UserID{user}},
			check: func(t *testing.T, i domain.Issue) {
				t.Helper()

				if i.Title != "New" || len(i.Tags) != 1 || len(i.Assignees) != 1 || !i.UpdatedAt.Equal(later) {
					t.Fatalf("Apply() = %+v", i)
				}
			},
		},
		{
			name:    "unarchive",
			status:  domain.StatusArchived,
			changes: domain.IssueChanges{Status: new(domain.StatusOpen)},
			check: func(t *testing.T, i domain.Issue) {
				t.Helper()

				if i.Status != domain.StatusOpen {
					t.Fatalf("Status = %q, want open", i.Status)
				}
			},
		},
		{
			name:    "archived rejects title",
			status:  domain.StatusArchived,
			changes: domain.IssueChanges{Title: new("New")},
			wantErr: domain.ErrArchived,
		},
		{
			name:    "archived rejects status plus tags",
			status:  domain.StatusArchived,
			changes: domain.IssueChanges{Status: new(domain.StatusOpen), Tags: &[]domain.TagID{}},
			wantErr: domain.ErrArchived,
		},
		{
			name:    "archived rejects status plus storage items",
			status:  domain.StatusArchived,
			changes: domain.IssueChanges{Status: new(domain.StatusOpen), StorageItems: &[]domain.StorageItemID{}},
			wantErr: domain.ErrArchived,
		},
		{
			name:       "invalid status",
			status:     domain.StatusOpen,
			changes:    domain.IssueChanges{Status: new(domain.IssueStatus("done")), Title: new("x")},
			wantFields: map[string]string{"status": domain.FieldInvalid},
		},
		{
			name:    "no changes keeps timestamp",
			status:  domain.StatusOpen,
			changes: domain.IssueChanges{},
			check: func(t *testing.T, i domain.Issue) {
				t.Helper()

				if !i.UpdatedAt.Equal(testNow) {
					t.Fatalf("UpdatedAt = %v, want %v", i.UpdatedAt, testNow)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			issue, err := domain.NewIssue(domain.NewIssueParams{Title: "Old", Status: tt.status, Now: testNow})
			if err != nil {
				t.Fatal(err)
			}

			before := issue
			err = issue.Apply(tt.changes, later)

			if !checkApplyErr(t, err, tt.wantErr, tt.wantFields) {
				if issue.Title != before.Title || issue.Status != before.Status {
					t.Fatalf("failed Apply() changed issue to %+v", issue)
				}

				return
			}

			tt.check(t, issue)
		})
	}
}

func TestIssue_ApplyStorageItems(t *testing.T) {
	t.Parallel()

	later := testNow.Add(time.Hour)
	item := domain.StorageItemID(uuid.New())

	issue, err := domain.NewIssue(domain.NewIssueParams{Title: "Old", StorageItems: []domain.StorageItemID{item, item}, Now: testNow})
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(issue.StorageItems, []domain.StorageItemID{item}) {
		t.Fatalf("NewIssue() storage items = %v", issue.StorageItems)
	}

	if err := issue.Apply(domain.IssueChanges{StorageItems: &[]domain.StorageItemID{}}, later); err != nil {
		t.Fatal(err)
	}

	if len(issue.StorageItems) != 0 || !issue.UpdatedAt.Equal(later) {
		t.Fatalf("Apply() = %+v", issue)
	}
}

// checkApplyErr asserts err against the expectation and reports whether the
// call was expected to succeed.
func checkApplyErr(t *testing.T, err, wantErr error, fields map[string]string) bool {
	t.Helper()

	switch {
	case wantErr != nil:
		if !errors.Is(err, wantErr) {
			t.Fatalf("Apply() error = %v, want %v", err, wantErr)
		}

		return false
	case fields != nil:
		wantFields(t, err, fields)

		return false
	case err != nil:
		t.Fatalf("Apply() error = %v", err)
	}

	return true
}
