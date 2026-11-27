package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/adapter/storage/memory"
	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
	"nilswitt.dev/issuetrax/internal/core/service"
)

var testNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// testActor acts in tests that do not care who made a change.
var testActor = domain.UserID(uuid.MustParse("00000000-0000-0000-0000-0000000000ac"))

// fixture wires all services to one fresh in-memory store.
type fixture struct {
	repos       service.Repositories
	projects    *service.ProjectService
	issues      *service.IssueService
	tags        *service.TagService
	comments    *service.CommentService
	attachments *service.AttachmentService
	storages    *service.StorageService
	items       *service.GenericStorageItemService
	devices     *service.NetworkDeviceService
	users       *service.UserService
	// changes records every change the services announced.
	changes *changeRecorder
}

// changeRecorder is a port.ChangeNotifier that remembers what it was told.
type changeRecorder struct {
	mu   sync.Mutex
	seen []domain.Change
}

func (r *changeRecorder) Notify(_ context.Context, c domain.Change) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seen = append(r.seen, c)
}

// take returns the recorded changes and forgets them.
func (r *changeRecorder) take() []domain.Change {
	r.mu.Lock()
	defer r.mu.Unlock()

	seen := r.seen
	r.seen = nil

	return seen
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	store := memory.NewStore()
	repos := service.Repositories{
		Tx:                  store,
		Projects:            memory.NewProjectRepository(store),
		Issues:              memory.NewIssueRepository(store),
		Tags:                memory.NewTagRepository(store),
		Comments:            memory.NewCommentRepository(store),
		Attachments:         memory.NewAttachmentRepository(store),
		Storages:            memory.NewStorageRepository(store),
		GenericStorageItems: memory.NewGenericStorageItemRepository(store),
		NetworkDevices:      memory.NewNetworkDeviceRepository(store),
		Users:               memory.NewUserRepository(store),
		Sessions:            memory.NewSessionRepository(store),
		APIKeys:             memory.NewAPIKeyRepository(store),
		Audit:               memory.NewAuditRepository(store),
	}
	clock := service.WithClock(func() time.Time { return testNow })
	changes := &changeRecorder{}
	notify := service.WithChangeNotifier(changes)

	return &fixture{
		repos:       repos,
		projects:    service.NewProjectService(repos, clock, notify),
		issues:      service.NewIssueService(repos, clock, notify),
		tags:        service.NewTagService(repos, clock, notify),
		comments:    service.NewCommentService(repos, clock, notify),
		attachments: service.NewAttachmentService(repos, clock, notify),
		storages:    service.NewStorageService(repos, clock, notify),
		items:       service.NewGenericStorageItemService(repos, clock, notify),
		devices:     service.NewNetworkDeviceService(repos, clock, notify),
		users:       service.NewUserService(repos),
		changes:     changes,
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

	p, err := f.projects.Create(t.Context(), testActor, port.CreateProjectInput{Key: key, Name: name})
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
