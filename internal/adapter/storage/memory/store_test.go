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
			Tx:                  s,
			Projects:            memory.NewProjectRepository(s),
			Issues:              memory.NewIssueRepository(s),
			Tags:                memory.NewTagRepository(s),
			Comments:            memory.NewCommentRepository(s),
			Attachments:         memory.NewAttachmentRepository(s),
			Storages:            memory.NewStorageRepository(s),
			GenericStorageItems: memory.NewGenericStorageItemRepository(s),
			NetworkDevices:      memory.NewNetworkDeviceRepository(s),
			Users:               memory.NewUserRepository(s),
			Sessions:            memory.NewSessionRepository(s),
			APIKeys:             memory.NewAPIKeyRepository(s),
			Audit:               memory.NewAuditRepository(s),
		}
	})
}
