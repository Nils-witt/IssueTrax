package memory_test

import (
	"testing"

	"nilswitt.dev/issuetrax/internal/adapter/storage/memory"
	"nilswitt.dev/issuetrax/internal/adapter/storage/storagetest"
	"nilswitt.dev/issuetrax/internal/core/service"
)

func TestContract(t *testing.T) {
	t.Parallel()

	storagetest.Run(t, func(t *testing.T) service.Repositories {
		t.Helper()

		s := memory.NewStore()

		return service.Repositories{
			Tx:       s,
			Projects: memory.NewProjectRepository(s),
			Issues:   memory.NewIssueRepository(s),
			Tags:     memory.NewTagRepository(s),
			Users:    memory.NewUserRepository(s),
		}
	})
}
