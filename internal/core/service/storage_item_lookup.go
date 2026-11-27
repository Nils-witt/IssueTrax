package service

import (
	"context"
	"fmt"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that StorageItemLookupService satisfies the driving port.
var _ port.StorageItemLookupService = (*StorageItemLookupService)(nil)

// StorageItemLookupService implements port.StorageItemLookupService.
type StorageItemLookupService struct {
	repos Repositories
}

// NewStorageItemLookupService returns a StorageItemLookupService backed by repos.
func NewStorageItemLookupService(repos Repositories) *StorageItemLookupService {
	return &StorageItemLookupService{repos: repos}
}

// Search returns one page of items of any kind across all storages.
func (s *StorageItemLookupService) Search(
	ctx context.Context, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.StorageItem], error) {
	items, err := s.repos.StorageItems.Search(ctx, filter, page)
	if err != nil {
		return domain.Page[domain.StorageItem]{}, fmt.Errorf("search storage items: %w", err)
	}

	return items, nil
}
