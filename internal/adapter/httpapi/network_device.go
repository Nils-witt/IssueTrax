package httpapi

import (
	"net/http"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

type ipConfigResponse struct {
	Mode string `json:"mode"`
	// Address, PrefixLength and Gateway are null unless set (static mode).
	Address      *string  `json:"address"`
	PrefixLength *int     `json:"prefixLength"`
	Gateway      *string  `json:"gateway"`
	DNSServers   []string `json:"dnsServers"`
}

type networkInterfaceResponse struct {
	Name string `json:"name"`
	MAC  string `json:"mac"`
	Type string `json:"type"`
	SSID string `json:"ssid"`
	// NativeVLAN is null when the interface has no untagged VLAN.
	NativeVLAN  *int             `json:"nativeVlan"`
	TaggedVLANs []int            `json:"taggedVlans"`
	IPv4        ipConfigResponse `json:"ipv4"`
	IPv6        ipConfigResponse `json:"ipv6"`
}

type networkDeviceResponse struct {
	ID                  string                     `json:"id"`
	StorageID           string                     `json:"storageId"`
	Name                string                     `json:"name"`
	Description         string                     `json:"description"`
	Role                string                     `json:"role"`
	Hostname            string                     `json:"hostname"`
	Manufacturer        string                     `json:"manufacturer"`
	Model               string                     `json:"model"`
	SerialNumber        string                     `json:"serialNumber"`
	PrimaryInterface    networkInterfaceResponse   `json:"primaryInterface"`
	SecondaryInterfaces []networkInterfaceResponse `json:"secondaryInterfaces"`
	CreatedAt           time.Time                  `json:"createdAt"`
	UpdatedAt           time.Time                  `json:"updatedAt"`
	// DeletedAt is null unless the device is deleted.
	DeletedAt *time.Time `json:"deletedAt"`
}

func toNetworkDeviceResponse(d domain.NetworkDevice) networkDeviceResponse {
	secondary := make([]networkInterfaceResponse, 0, len(d.Secondary))
	for _, n := range d.Secondary {
		secondary = append(secondary, toNetworkInterfaceResponse(n))
	}

	return networkDeviceResponse{
		ID:                  d.ID.String(),
		StorageID:           d.StorageID.String(),
		Name:                d.Name,
		Description:         d.Description,
		Role:                string(d.Role),
		Hostname:            d.Hostname,
		Manufacturer:        d.Manufacturer,
		Model:               d.Model,
		SerialNumber:        d.SerialNumber,
		PrimaryInterface:    toNetworkInterfaceResponse(d.Primary),
		SecondaryInterfaces: secondary,
		CreatedAt:           d.CreatedAt,
		UpdatedAt:           d.UpdatedAt,
		DeletedAt:           d.DeletedAt,
	}
}

func toNetworkInterfaceResponse(n domain.NetworkInterface) networkInterfaceResponse {
	resp := networkInterfaceResponse{
		Name:        n.Name,
		MAC:         n.MAC,
		Type:        string(n.Type),
		SSID:        n.SSID,
		TaggedVLANs: append([]int{}, n.TaggedVLANs...),
		IPv4:        toIPConfigResponse(n.IPv4),
		IPv6:        toIPConfigResponse(n.IPv6),
	}

	if n.NativeVLAN != 0 {
		v := n.NativeVLAN
		resp.NativeVLAN = &v
	}

	return resp
}

func toIPConfigResponse(c domain.IPConfig) ipConfigResponse {
	resp := ipConfigResponse{Mode: string(c.Mode), DNSServers: make([]string, 0, len(c.DNSServers))}

	if c.Address.IsValid() {
		addr, bits := c.Address.Addr().String(), c.Address.Bits()
		resp.Address, resp.PrefixLength = &addr, &bits
	}

	if c.Gateway.IsValid() {
		gw := c.Gateway.String()
		resp.Gateway = &gw
	}

	for _, d := range c.DNSServers {
		resp.DNSServers = append(resp.DNSServers, d.String())
	}

	return resp
}

type ipConfigRequest struct {
	Mode         string   `json:"mode"`
	Address      *string  `json:"address"`
	PrefixLength *int     `json:"prefixLength"`
	Gateway      *string  `json:"gateway"`
	DNSServers   []string `json:"dnsServers"`
}

func (c ipConfigRequest) params() domain.IPConfigParams {
	return domain.IPConfigParams{
		Mode:         domain.IPMode(c.Mode),
		Address:      deref(c.Address),
		PrefixLength: c.PrefixLength,
		Gateway:      deref(c.Gateway),
		DNSServers:   c.DNSServers,
	}
}

type networkInterfaceRequest struct {
	Name        string          `json:"name"`
	MAC         string          `json:"mac"`
	Type        string          `json:"type"`
	SSID        *string         `json:"ssid"`
	NativeVLAN  *int            `json:"nativeVlan"`
	TaggedVLANs []int           `json:"taggedVlans"`
	IPv4        ipConfigRequest `json:"ipv4"`
	IPv6        ipConfigRequest `json:"ipv6"`
}

func (n networkInterfaceRequest) params() domain.NetworkInterfaceParams {
	return domain.NetworkInterfaceParams{
		Name:        n.Name,
		MAC:         n.MAC,
		Type:        domain.NetworkInterfaceType(n.Type),
		SSID:        deref(n.SSID),
		NativeVLAN:  n.NativeVLAN,
		TaggedVLANs: n.TaggedVLANs,
		IPv4:        n.IPv4.params(),
		IPv6:        n.IPv6.params(),
	}
}

func interfaceParams(reqs []networkInterfaceRequest) []domain.NetworkInterfaceParams {
	out := make([]domain.NetworkInterfaceParams, 0, len(reqs))
	for _, n := range reqs {
		out = append(out, n.params())
	}

	return out
}

type createNetworkDeviceRequest struct {
	StorageID           string                    `json:"storageId"`
	Name                string                    `json:"name"`
	Description         string                    `json:"description"`
	Role                string                    `json:"role"`
	Hostname            string                    `json:"hostname"`
	Manufacturer        string                    `json:"manufacturer"`
	Model               string                    `json:"model"`
	SerialNumber        string                    `json:"serialNumber"`
	PrimaryInterface    networkInterfaceRequest   `json:"primaryInterface"`
	SecondaryInterfaces []networkInterfaceRequest `json:"secondaryInterfaces"`
}

type updateNetworkDeviceRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	Role         *string `json:"role"`
	Hostname     *string `json:"hostname"`
	Manufacturer *string `json:"manufacturer"`
	Model        *string `json:"model"`
	SerialNumber *string `json:"serialNumber"`
	// StorageID moves the device when present.
	StorageID *string `json:"storageId"`
	// PrimaryInterface and SecondaryInterfaces each replace what is stored
	// when present.
	PrimaryInterface    *networkInterfaceRequest   `json:"primaryInterface"`
	SecondaryInterfaces *[]networkInterfaceRequest `json:"secondaryInterfaces"`
}

// GET /api/network-devices?q=&deleted=&limit=&cursor=.
func (h *Handler) searchNetworkDevices(w http.ResponseWriter, r *http.Request, _ domain.User) {
	searchItems(h, w, r, h.networkDevices, toNetworkDeviceResponse)
}

// GET /api/storages/{id}/network-devices?recursive=&deleted=&limit=&cursor=.
func (h *Handler) listNetworkDevicesInStorage(w http.ResponseWriter, r *http.Request, _ domain.User) {
	listItemsInStorage(h, w, r, h.networkDevices, toNetworkDeviceResponse)
}

// POST /api/network-devices.
func (h *Handler) createNetworkDevice(w http.ResponseWriter, r *http.Request, user domain.User) {
	var req createNetworkDeviceRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	storage, err := parseStorageID(req.StorageID, "storageId")
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	device, err := h.networkDevices.Create(r.Context(), user.ID, port.CreateNetworkDeviceInput{
		StorageID:    *storage,
		Name:         req.Name,
		Description:  req.Description,
		Role:         domain.NetworkRole(req.Role),
		Hostname:     req.Hostname,
		Manufacturer: req.Manufacturer,
		Model:        req.Model,
		SerialNumber: req.SerialNumber,
		Primary:      req.PrimaryInterface.params(),
		Secondary:    interfaceParams(req.SecondaryInterfaces),
	})
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toNetworkDeviceResponse(device))
}

// GET /api/network-devices/{id}. Deleted devices are returned too.
func (h *Handler) getNetworkDevice(w http.ResponseWriter, r *http.Request, _ domain.User) {
	getItem(h, w, r, h.networkDevices, toNetworkDeviceResponse)
}

// PATCH /api/network-devices/{id}.
func (h *Handler) updateNetworkDevice(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := storageItemPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageItemNotFound)
		return
	}

	var req updateNetworkDeviceRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	in := port.UpdateNetworkDeviceInput{
		Name: req.Name, Description: req.Description, Hostname: req.Hostname,
		Manufacturer: req.Manufacturer, Model: req.Model, SerialNumber: req.SerialNumber,
	}

	if req.Role != nil {
		role := domain.NetworkRole(*req.Role)
		in.Role = &role
	}

	if req.StorageID != nil {
		storage, err := parseStorageID(*req.StorageID, "storageId")
		if err != nil {
			h.writeDomainError(w, r, err)
			return
		}

		in.StorageID = storage
	}

	if req.PrimaryInterface != nil {
		primary := req.PrimaryInterface.params()
		in.Primary = &primary
	}

	if req.SecondaryInterfaces != nil {
		secondary := interfaceParams(*req.SecondaryInterfaces)
		in.Secondary = &secondary
	}

	device, err := h.networkDevices.Update(r.Context(), user.ID, id, in)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toNetworkDeviceResponse(device))
}

// DELETE /api/network-devices/{id} marks the device deleted.
func (h *Handler) deleteNetworkDevice(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := storageItemPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageItemNotFound)
		return
	}

	if err := h.networkDevices.Delete(r.Context(), user.ID, id); err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/network-devices/{id}/restore.
func (h *Handler) restoreNetworkDevice(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := storageItemPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageItemNotFound)
		return
	}

	device, err := h.networkDevices.Restore(r.Context(), user.ID, id)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toNetworkDeviceResponse(device))
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
