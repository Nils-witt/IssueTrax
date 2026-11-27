package domain

import (
	"fmt"
	"slices"
	"time"
)

// NetworkDevice is a storage item of kind StorageItemKindNetworkDevice: a
// device with exactly one primary and any number of secondary interfaces.
type NetworkDevice struct {
	StorageItem
	Role NetworkRole
	// Hostname, Manufacturer, Model and SerialNumber may be empty.
	Hostname     string
	Manufacturer string
	Model        string
	SerialNumber string
	Primary      NetworkInterface
	Secondary    []NetworkInterface
}

// NewNetworkDeviceParams are the inputs for NewNetworkDevice.
type NewNetworkDeviceParams struct {
	ID           StorageItemID
	StorageID    StorageID
	Name         string
	Description  string
	Role         NetworkRole
	Hostname     string
	Manufacturer string
	Model        string
	SerialNumber string
	Primary      NetworkInterfaceParams
	Secondary    []NetworkInterfaceParams
	Now          time.Time
}

// NewNetworkDevice validates p and builds a NetworkDevice. It does not
// check that the storage exists; that needs storage. Field errors use the
// API field names, e.g. "secondaryInterfaces[1].ipv4.gateway".
func NewNetworkDevice(p NewNetworkDeviceParams) (NetworkDevice, error) {
	f := fieldErrors{}

	device := NetworkDevice{
		ID:           p.ID,
		StorageID:    p.StorageID,
		Kind:         StorageItemKindNetworkDevice,
		Name:         checkRequiredText(f, "name", p.Name, maxProjectNameRunes),
		Description:  checkDescription(f, "description", p.Description),
		CreatedAt:    p.Now,
		UpdatedAt:    p.Now,
		Role:         checkRole(f, "role", p.Role),
		Hostname:     checkHostname(f, "hostname", p.Hostname),
		Manufacturer: checkOptionalText(f, "manufacturer", p.Manufacturer, maxShortTextRunes),
		Model:        checkOptionalText(f, "model", p.Model, maxShortTextRunes),
		SerialNumber: checkOptionalText(f, "serialNumber", p.SerialNumber, maxShortTextRunes),
		Primary:      checkInterface(f, "primaryInterface", p.Primary),
		Secondary:    checkInterfaces(f, p.Secondary),
	}

	if err := f.err(); err != nil {
		return NetworkDevice{}, err
	}

	return device, nil
}

// NetworkDeviceChanges lists the mutable fields; nil means unchanged.
// StorageID moves the device. Primary and Secondary each replace what is
// stored, so swapping them makes a secondary interface primary.
type NetworkDeviceChanges struct {
	Name         *string
	Description  *string
	StorageID    *StorageID
	Role         *NetworkRole
	Hostname     *string
	Manufacturer *string
	Model        *string
	SerialNumber *string
	Primary      *NetworkInterfaceParams
	Secondary    *[]NetworkInterfaceParams
}

// Apply validates and applies c. On error d is left unchanged. A deleted
// device fails with ErrStorageItemDeleted.
func (d *NetworkDevice) Apply(c NetworkDeviceChanges, now time.Time) error {
	if d.IsDeleted() {
		return ErrStorageItemDeleted
	}

	f := fieldErrors{}
	next := *d

	if c.Name != nil {
		next.Name = checkRequiredText(f, "name", *c.Name, maxProjectNameRunes)
	}

	if c.Description != nil {
		next.Description = checkDescription(f, "description", *c.Description)
	}

	if c.StorageID != nil {
		next.StorageID = *c.StorageID
	}

	next.applyDetails(f, c)

	if c.Primary != nil {
		next.Primary = checkInterface(f, "primaryInterface", *c.Primary)
	}

	if c.Secondary != nil {
		next.Secondary = checkInterfaces(f, *c.Secondary)
	}

	if err := f.err(); err != nil {
		return err
	}

	if !next.sameContent(*d) {
		next.UpdatedAt = now
	}

	*d = next

	return nil
}

// applyDetails applies the role and identification fields of c to d.
func (d *NetworkDevice) applyDetails(f fieldErrors, c NetworkDeviceChanges) {
	if c.Role != nil {
		d.Role = checkRole(f, "role", *c.Role)
	}

	if c.Hostname != nil {
		d.Hostname = checkHostname(f, "hostname", *c.Hostname)
	}

	if c.Manufacturer != nil {
		d.Manufacturer = checkOptionalText(f, "manufacturer", *c.Manufacturer, maxShortTextRunes)
	}

	if c.Model != nil {
		d.Model = checkOptionalText(f, "model", *c.Model, maxShortTextRunes)
	}

	if c.SerialNumber != nil {
		d.SerialNumber = checkOptionalText(f, "serialNumber", *c.SerialNumber, maxShortTextRunes)
	}
}

// sameContent reports whether d and o have the same mutable fields.
func (d NetworkDevice) sameContent(o NetworkDevice) bool {
	return d.Name == o.Name && d.Description == o.Description && d.StorageID == o.StorageID && d.Role == o.Role &&
		d.Hostname == o.Hostname && d.Manufacturer == o.Manufacturer && d.Model == o.Model &&
		d.SerialNumber == o.SerialNumber && d.Primary.Equal(o.Primary) &&
		slices.EqualFunc(d.Secondary, o.Secondary, NetworkInterface.Equal)
}

func checkInterfaces(f fieldErrors, ps []NetworkInterfaceParams) []NetworkInterface {
	out := make([]NetworkInterface, 0, len(ps))
	for i, p := range ps {
		out = append(out, checkInterface(f, fmt.Sprintf("secondaryInterfaces[%d]", i), p))
	}

	return out
}
