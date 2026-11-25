package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/adapter/storage/memory"
	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
	"nilswitt.dev/issuetrax/internal/core/service"
)

var testNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// fixture wires all services to one fresh in-memory store.
type fixture struct {
	repos    service.Repositories
	projects *service.ProjectService
	issues   *service.IssueService
	tags     *service.TagService
	comments *service.CommentService
	users    *service.UserService
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	store := memory.NewStore()
	repos := service.Repositories{
		Tx:       store,
		Projects: memory.NewProjectRepository(store),
		Issues:   memory.NewIssueRepository(store),
		Tags:     memory.NewTagRepository(store),
		Comments: memory.NewCommentRepository(store),
		Users:    memory.NewUserRepository(store),
		Sessions: memory.NewSessionRepository(store),
	}
	clock := service.WithClock(func() time.Time { return testNow })

	return &fixture{
		repos:    repos,
		projects: service.NewProjectService(repos, clock),
		issues:   service.NewIssueService(repos, clock),
		tags:     service.NewTagService(repos),
		comments: service.NewCommentService(repos, clock),
		users:    service.NewUserService(repos),
	}
}

func (f *fixture) addUser(t *testing.T, name string) domain.User {
	t.Helper()

	u, err := domain.NewUser(domain.UserID(uuid.New()), domain.Identity{Issuer: "iss", Subject: name, DisplayName: name}, testNow)
	if err != nil {
		t.Fatal(err)
	}

	if err := f.repos.Users.Create(t.Context(), u); err != nil {
		t.Fatal(err)
	}

	return u
}

func (f *fixture) addProject(t *testing.T, key, name string) domain.Project {
	t.Helper()

	p, err := f.projects.Create(t.Context(), port.CreateProjectInput{Key: key, Name: name})
	if err != nil {
		t.Fatalf("create project %s: %v", key, err)
	}

	return p
}

func (f *fixture) addIssue(t *testing.T, actor domain.UserID, key domain.ProjectKey, in port.CreateIssueInput) port.IssueView {
	t.Helper()

	v, err := f.issues.Create(t.Context(), actor, key, in)
	if err != nil {
		t.Fatalf("create issue %q: %v", in.Title, err)
	}

	return v
}

func firstPage(t *testing.T, limit int) domain.PageRequest {
	t.Helper()

	p, err := domain.NewPageRequest(limit, "")
	if err != nil {
		t.Fatal(err)
	}

	return p
}

// wantCode fails unless err is a domain error with code.
func wantCode(t *testing.T, err error, code string) {
	t.Helper()

	de, ok := errors.AsType[*domain.Error](err)
	if !ok || de.Code != code {
		t.Fatalf("error = %v, want code %q", err, code)
	}
}

// wantField fails unless err is a validation error flagging field with code.
func wantField(t *testing.T, err error, field, code string) {
	t.Helper()

	wantCode(t, err, domain.CodeValidationFailed)

	de, _ := errors.AsType[*domain.Error](err)
	if de.Fields[field] != code {
		t.Fatalf("fields = %v, want %s: %s", de.Fields, field, code)
	}
}
