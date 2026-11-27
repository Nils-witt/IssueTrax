package port

import (
	"context"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// CreateNetworkDeviceInput carries the data for a new network device.
type CreateNetworkDeviceInput struct {
	StorageID    domain.StorageID
	Name         string
	Description  string
	Role         domain.NetworkRole
	Hostname     string
	Manufacturer string
	Model        string
	SerialNumber string
	Primary      domain.NetworkInterfaceParams
	Secondary    []domain.NetworkInterfaceParams
}

// UpdateNetworkDeviceInput is a partial update; nil fields stay unchanged.
// StorageID moves the device. Primary and Secondary each replace what is
// stored; sending both with swapped entries changes the primary interface.
type UpdateNetworkDeviceInput struct {
	Name         *string
	Description  *string
	StorageID    *domain.StorageID
	Role         *domain.NetworkRole
	Hostname     *string
	Manufacturer *string
	Model        *string
	SerialNumber *string
	Primary      *domain.NetworkInterfaceParams
	Secondary    *[]domain.NetworkInterfaceParams
}

// NetworkDeviceService is the driving port for network device use cases.
// Search matches name, description, hostname, serial number and the MACs
// of all interfaces.
type NetworkDeviceService interface {
	StorageItemService[domain.NetworkDevice]
	Create(ctx context.Context, actor domain.UserID, in CreateNetworkDeviceInput) (domain.NetworkDevice, error)
	Update(
		ctx context.Context, actor domain.UserID, id domain.StorageItemID, in UpdateNetworkDeviceInput,
	) (domain.NetworkDevice, error)
}

// NetworkDeviceRepository is the driven port for persisting network
// devices. It only sees items of kind domain.StorageItemKindNetworkDevice.
// Search matches filter.Query, ignoring case, against name, description,
// hostname, serial number and the MACs of all interfaces.
//
// Update stores every mutable field including DeletedAt and the
// interfaces, and returns domain.ErrStorageItemNotFound for an unknown
// device.
type NetworkDeviceRepository interface {
	StorageItemReader[domain.NetworkDevice]
	Create(ctx context.Context, device domain.NetworkDevice) error
	Update(ctx context.Context, device domain.NetworkDevice) error
}
