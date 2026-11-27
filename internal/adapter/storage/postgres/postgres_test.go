//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"nilswitt.dev/issuetrax/internal/adapter/storage/postgres"
	"nilswitt.dev/issuetrax/internal/adapter/storage/storagetest"
	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
	"nilswitt.dev/issuetrax/internal/core/service"
)

const templateDB = "issuetrax_template"

var (
	// baseURL points at the container; tests swap in their own database name.
	baseURL  *url.URL
	errSetup error
	createMu sync.Mutex
)

// TestMain starts one Postgres container and migrates a template database;
// every test then gets a private copy (CREATE DATABASE ... TEMPLATE), which
// takes milliseconds.
func TestMain(m *testing.M) { os.Exit(run(m)) }

func run(m *testing.M) int {
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, "postgres:18-alpine",
		tcpostgres.WithDatabase(templateDB),
		tcpostgres.WithUsername("issuetrax"),
		tcpostgres.WithPassword("issuetrax"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		errSetup = fmt.Errorf("start postgres container: %w", err)

		return m.Run()
	}

	defer func() { _ = testcontainers.TerminateContainer(ctr) }()

	errSetup = setup(ctx, ctr)

	return m.Run()
}

func setup(ctx context.Context, ctr *tcpostgres.PostgresContainer) error {
	raw, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fmt.Errorf("connection string: %w", err)
	}

	if err := postgres.Migrate(ctx, raw); err != nil {
		return err
	}

	baseURL, err = url.Parse(raw)

	return err
}

func databaseURL(name string) string {
	u := *baseURL
	u.Path = "/" + name

	return u.String()
}

// newDB returns a connection to a fresh, migrated database.
func newDB(t *testing.T) *postgres.DB {
	t.Helper()

	if errSetup != nil {
		t.Skipf("postgres unavailable (is Docker running?): %v", errSetup)
	}

	ctx := t.Context()
	name := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")

	func() {
		// Copying a template fails while another copy of it is in progress.
		createMu.Lock()
		defer createMu.Unlock()

		admin, err := pgx.Connect(ctx, databaseURL("postgres"))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = admin.Close(context.WithoutCancel(ctx)) }()

		// name is generated above, so it is a safe identifier.
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+name+" TEMPLATE "+templateDB); err != nil {
			t.Fatal(err)
		}
	}()

	db, err := postgres.Open(ctx, databaseURL(name))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(db.Close)

	return db
}

func repositories(db *postgres.DB) service.Repositories {
	return service.Repositories{
		Tx:               db,
		Projects:         postgres.NewProjectRepository(db),
		Issues:           postgres.NewIssueRepository(db),
		Tags:             postgres.NewTagRepository(db),
		Comments:         postgres.NewCommentRepository(db),
		Attachments:      postgres.NewAttachmentRepository(db),
		Storages:         postgres.NewStorageRepository(db),
		ItemTypes:        postgres.NewItemTypeRepository(db),
		StorageItems:     postgres.NewStorageItemRepository(db),
		AssetTagCounters: postgres.NewAssetTagCounterRepository(db),
		Users:            postgres.NewUserRepository(db),
		Sessions:         postgres.NewSessionRepository(db),
		APIKeys:          postgres.NewAPIKeyRepository(db),
		Audit:            postgres.NewAuditRepository(db),
	}
}

func TestContract(t *testing.T) {
	t.Parallel()

	storagetest.Run(t, func(t *testing.T) service.Repositories {
		t.Helper()

		return repositories(newDB(t))
	})
}

func TestMigrateIsIdempotent(t *testing.T) {
	t.Parallel()

	if errSetup != nil {
		t.Skipf("postgres unavailable: %v", errSetup)
	}

	if err := postgres.Migrate(t.Context(), databaseURL(templateDB)); err != nil {
		t.Fatalf("second Migrate() = %v", err)
	}
}

// TestConcurrentIssueCreation checks that parallel creates get distinct,
// gap-free numbers and converge on one row per new tag name.
func TestConcurrentIssueCreation(t *testing.T) {
	t.Parallel()

	repos := repositories(newDB(t))
	ctx := t.Context()

	user, err := domain.NewUser(domain.UserID(uuid.New()), domain.Identity{Issuer: "iss", Subject: "ada"}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	if err := repos.Users.Create(ctx, user); err != nil {
		t.Fatal(err)
	}

	if _, err := service.NewProjectService(repos).Create(ctx, user.ID, port.CreateProjectInput{Key: "TRAX", Name: "IssueTrax"}); err != nil {
		t.Fatal(err)
	}

	issues := service.NewIssueService(repos)

	const n = 20

	numbers := make([]int64, n)

	var wg sync.WaitGroup

	for i := range n {
		wg.Go(func() {
			v, err := issues.Create(ctx, user.ID, "TRAX", port.CreateIssueInput{Title: "parallel", Tags: []string{"shared"}})
			if err != nil {
				t.Errorf("Create() = %v", err)
				return
			}

			numbers[i] = v.Issue.Number
		})
	}

	wg.Wait()

	slices.Sort(numbers)

	for i, got := range numbers {
		if got != int64(i+1) {
			t.Fatalf("numbers = %v, want 1..%d", numbers, n)
		}
	}

	tags, err := service.NewTagService(repos).List(ctx, "TRAX")
	if err != nil || len(tags) != 1 {
		t.Fatalf("tags = %+v, %v; want one shared tag", tags, err)
	}
}
