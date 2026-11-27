package service

import (
	"context"
	"fmt"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that NetworkDeviceService satisfies the driving port.
var _ port.NetworkDeviceService = (*NetworkDeviceService)(nil)

// NetworkDeviceService implements port.NetworkDeviceService.
type NetworkDeviceService struct {
	repos Repositories
	opts  options
	kind  itemKind[domain.NetworkDevice]
}

// NewNetworkDeviceService returns a NetworkDeviceService backed by repos.
func NewNetworkDeviceService(repos Repositories, opts ...Option) *NetworkDeviceService {
	o := newOptions(opts)

	return &NetworkDeviceService{repos: repos, opts: o, kind: itemKind[domain.NetworkDevice]{
		repos: repos, opts: o, repo: repos.NetworkDevices,
		base:  func(d *domain.NetworkDevice) *domain.StorageItem { return &d.StorageItem },
		event: networkDeviceEvent,
	}}
}

// Create validates and stores a new device in in.StorageID.
func (s *NetworkDeviceService) Create(
	ctx context.Context, actor domain.UserID, in port.CreateNetworkDeviceInput,
) (domain.NetworkDevice, error) {
	device, err := domain.NewNetworkDevice(domain.NewNetworkDeviceParams{
		ID:           domain.StorageItemID(s.opts.newID()),
		StorageID:    in.StorageID,
		Name:         in.Name,
		Description:  in.Description,
		Role:         in.Role,
		Hostname:     in.Hostname,
		Manufacturer: in.Manufacturer,
		Model:        in.Model,
		SerialNumber: in.SerialNumber,
		Primary:      in.Primary,
		Secondary:    in.Secondary,
		Now:          s.opts.now(),
	})
	if err != nil {
		return domain.NetworkDevice{}, err
	}

	err = s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := checkItemStorage(ctx, s.repos, device.StorageID); err != nil {
			return err
		}

		if err := s.repos.NetworkDevices.Create(ctx, device); err != nil {
			return fmt.Errorf("create network device %q: %w", device.Name, err)
		}

		changes := domain.AuditChanges{}
		changes.Initial("name", device.Name)
		changes.InitialText("description", device.Description)
		changes.Initial("storage", device.StorageID.String())
		changes.Initial("role", string(device.Role))
		changes.Initial("hostname", device.Hostname)
		changes.Initial("manufacturer", device.Manufacturer)
		changes.Initial("model", device.Model)
		changes.Initial("serialNumber", device.SerialNumber)
		changes.Initial("primaryInterface", device.Primary.String())
		changes.InitialList("secondaryInterfaces", interfaceStrings(device.Secondary))

		return record(ctx, s.repos, s.opts, networkDeviceEvent(actor, domain.AuditNetworkDeviceCreate, device, changes))
	})
	if err != nil {
		return domain.NetworkDevice{}, err
	}

	notifyStorageItems(ctx, s.opts)

	return device, nil
}

// Get returns the device with id, also when it is deleted.
func (s *NetworkDeviceService) Get(ctx context.Context, id domain.StorageItemID) (domain.NetworkDevice, error) {
	return s.kind.get(ctx, id)
}

// ListInStorage returns one page of the devices in storage. An unknown
// storage is not found.
func (s *NetworkDeviceService) ListInStorage(
	ctx context.Context, storage domain.StorageID, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.NetworkDevice], error) {
	return s.kind.listInStorage(ctx, storage, filter, page)
}

// Search returns one page of devices across all storages.
func (s *NetworkDeviceService) Search(
	ctx context.Context, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.NetworkDevice], error) {
	return s.kind.search(ctx, filter, page)
}

// Update edits and/or moves a device. Deleted devices fail with
// domain.ErrStorageItemDeleted.
func (s *NetworkDeviceService) Update(
	ctx context.Context, actor domain.UserID, id domain.StorageItemID, in port.UpdateNetworkDeviceInput,
) (domain.NetworkDevice, error) {
	var (
		device  domain.NetworkDevice
		changed bool
	)

	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if device, err = s.Get(ctx, id); err != nil {
			return err
		}

		before := device

		changes := domain.NetworkDeviceChanges{
			Name: in.Name, Description: in.Description, StorageID: in.StorageID, Role: in.Role,
			Hostname: in.Hostname, Manufacturer: in.Manufacturer, Model: in.Model, SerialNumber: in.SerialNumber,
			Primary: in.Primary, Secondary: in.Secondary,
		}
		if err := device.Apply(changes, s.opts.now()); err != nil {
			return err
		}

		if device.StorageID != before.StorageID {
			if err := checkItemStorage(ctx, s.repos, device.StorageID); err != nil {
				return err
			}
		}

		audit := domain.AuditChanges{}
		audit.Set("name", before.Name, device.Name)
		audit.Text("description", before.Description, device.Description)
		audit.Set("storage", before.StorageID.String(), device.StorageID.String())
		audit.Set("role", string(before.Role), string(device.Role))
		audit.Set("hostname", before.Hostname, device.Hostname)
		audit.Set("manufacturer", before.Manufacturer, device.Manufacturer)
		audit.Set("model", before.Model, device.Model)
		audit.Set("serialNumber", before.SerialNumber, device.SerialNumber)
		audit.Set("primaryInterface", before.Primary.String(), device.Primary.String())
		audit.List("secondaryInterfaces", interfaceStrings(before.Secondary), interfaceStrings(device.Secondary))

		if len(audit) == 0 {
			return nil
		}

		changed = true

		if err := s.repos.NetworkDevices.Update(ctx, device); err != nil {
			return fmt.Errorf("update network device %s: %w", device.ID, err)
		}

		return record(ctx, s.repos, s.opts, networkDeviceEvent(actor, domain.AuditNetworkDeviceUpdate, device, audit))
	})
	if err != nil {
		return domain.NetworkDevice{}, err
	}

	if changed {
		notifyStorageItems(ctx, s.opts)
	}

	return device, nil
}

// Delete marks a device deleted. An already deleted device fails with
// domain.ErrStorageItemDeleted.
func (s *NetworkDeviceService) Delete(ctx context.Context, actor domain.UserID, id domain.StorageItemID) error {
	_, err := s.kind.setDeleted(ctx, actor, id, domain.AuditNetworkDeviceDelete, (*domain.StorageItem).Delete)

	return err
}

// Restore undoes Delete. A device that is not deleted fails with
// domain.ErrStorageItemNotDeleted.
func (s *NetworkDeviceService) Restore(
	ctx context.Context, actor domain.UserID, id domain.StorageItemID,
) (domain.NetworkDevice, error) {
	return s.kind.setDeleted(ctx, actor, id, domain.AuditNetworkDeviceRestore, (*domain.StorageItem).Restore)
}

func interfaceStrings(ifaces []domain.NetworkInterface) []string {
	out := make([]string, 0, len(ifaces))
	for _, n := range ifaces {
		out = append(out, n.String())
	}

	return out
}

func networkDeviceEvent(
	actor domain.UserID, action domain.AuditAction, device domain.NetworkDevice, changes domain.AuditChanges,
) auditEvent {
	return auditEvent{
		actor: &actor, action: action, changes: changes,
		targetType: domain.AuditTargetNetworkDevice, targetID: device.ID.String(), targetLabel: device.Name,
	}
}
