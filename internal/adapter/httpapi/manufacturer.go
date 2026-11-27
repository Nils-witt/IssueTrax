package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// catalogRefJSON names a manufacturer or hardware model.
type catalogRefJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
}

func toManufacturerRef(m domain.Manufacturer) catalogRefJSON {
	return catalogRefJSON{ID: m.ID.String(), Name: m.Name, Deleted: m.IsDeleted()}
}

func toHardwareModelRef(m domain.HardwareModel) catalogRefJSON {
	return catalogRefJSON{ID: m.ID.String(), Name: m.Name, Deleted: m.IsDeleted()}
}

type manufacturerResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// ModelCount counts the live models, ItemCount the live items whose
	// hardware names the manufacturer.
	ModelCount int64     `json:"modelCount"`
	ItemCount  int64     `json:"itemCount"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	// DeletedAt is null unless the manufacturer is deleted.
	DeletedAt *time.Time `json:"deletedAt"`
}

func toManufacturerResponse(m domain.ManufacturerListItem) manufacturerResponse {
	return manufacturerResponse{
		ID: m.ID.String(), Name: m.Name, ModelCount: m.ModelCount, ItemCount: m.ItemCount,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, DeletedAt: m.DeletedAt,
	}
}

type hardwareModelResponse struct {
	ID             string `json:"id"`
	ManufacturerID string `json:"manufacturerId"`
	Name           string `json:"name"`
	// ItemCount counts the live items whose hardware names the model.
	ItemCount int64     `json:"itemCount"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// DeletedAt is null unless the model is deleted.
	DeletedAt *time.Time `json:"deletedAt"`
}

func toHardwareModelResponse(m domain.HardwareModelListItem) hardwareModelResponse {
	return hardwareModelResponse{
		ID: m.ID.String(), ManufacturerID: m.ManufacturerID.String(), Name: m.Name, ItemCount: m.ItemCount,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, DeletedAt: m.DeletedAt,
	}
}

// catalogNameRequest creates or renames a manufacturer or model.
type catalogNameRequest struct {
	Name string `json:"name"`
}

// catalogFilter reads q and includeDeleted.
func catalogFilter(r *http.Request) (port.CatalogFilter, error) {
	q := r.URL.Query()

	includeDeleted, err := boolParam(q, "includeDeleted")
	if err != nil {
		return port.CatalogFilter{}, err
	}

	return port.CatalogFilter{Query: strings.TrimSpace(q.Get("q")), IncludeDeleted: includeDeleted}, nil
}

// catalogRoutes registers the manufacturer and hardware model endpoints.
func (h *Handler) catalogRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/manufacturers", h.authed(h.listManufacturers))
	mux.Handle("POST /api/manufacturers", h.authed(h.createManufacturer))
	mux.Handle("GET /api/manufacturers/{id}", h.authed(h.getManufacturer))
	mux.Handle("PATCH /api/manufacturers/{id}", h.authed(h.updateManufacturer))
	mux.Handle("DELETE /api/manufacturers/{id}", h.authed(h.deleteManufacturer))
	mux.Handle("POST /api/manufacturers/{id}/restore", h.authed(h.restoreManufacturer))
	mux.Handle("GET /api/manufacturers/{id}/models", h.authed(h.listHardwareModels))
	mux.Handle("POST /api/manufacturers/{id}/models", h.authed(h.createHardwareModel))
	mux.Handle("GET /api/hardware-models/{id}", h.authed(h.getHardwareModel))
	mux.Handle("PATCH /api/hardware-models/{id}", h.authed(h.updateHardwareModel))
	mux.Handle("DELETE /api/hardware-models/{id}", h.authed(h.deleteHardwareModel))
	mux.Handle("POST /api/hardware-models/{id}/restore", h.authed(h.restoreHardwareModel))
}

// GET /api/manufacturers?q=&includeDeleted=&limit=&cursor= lists
// manufacturers by name.
func (h *Handler) listManufacturers(w http.ResponseWriter, r *http.Request, _ domain.User) {
	page, err := pageRequest(r.URL.Query())
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	filter, err := catalogFilter(r)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	found, err := h.manufacturers.List(r.Context(), filter, page)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toPageResponse(found, toManufacturerResponse))
}

// POST /api/manufacturers creates a manufacturer.
func (h *Handler) createManufacturer(w http.ResponseWriter, r *http.Request, user domain.User) {
	var req catalogNameRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	m, err := h.manufacturers.Create(r.Context(), user.ID, req.Name)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toManufacturerResponse(m))
}

// GET /api/manufacturers/{id}. Deleted manufacturers are returned too.
func (h *Handler) getManufacturer(w http.ResponseWriter, r *http.Request, _ domain.User) {
	id, ok := manufacturerPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrManufacturerNotFound)
		return
	}

	m, err := h.manufacturers.Get(r.Context(), id)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toManufacturerResponse(m))
}

// PATCH /api/manufacturers/{id} renames the manufacturer.
func (h *Handler) updateManufacturer(w http.ResponseWriter, r *http.Request, user domain.User) {
	withName(h, w, r, manufacturerPathID, domain.ErrManufacturerNotFound, http.StatusOK,
		func(id domain.ManufacturerID, name string) (manufacturerResponse, error) {
			m, err := h.manufacturers.Rename(r.Context(), user.ID, id, name)
			return toManufacturerResponse(m), err
		})
}

// DELETE /api/manufacturers/{id} marks the manufacturer deleted.
func (h *Handler) deleteManufacturer(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := manufacturerPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrManufacturerNotFound)
		return
	}

	if err := h.manufacturers.Delete(r.Context(), user.ID, id); err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/manufacturers/{id}/restore undoes a delete.
func (h *Handler) restoreManufacturer(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := manufacturerPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrManufacturerNotFound)
		return
	}

	m, err := h.manufacturers.Restore(r.Context(), user.ID, id)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toManufacturerResponse(m))
}

// GET /api/manufacturers/{id}/models?q=&includeDeleted=&limit=&cursor=
// lists the manufacturer's models by name.
func (h *Handler) listHardwareModels(w http.ResponseWriter, r *http.Request, _ domain.User) {
	id, ok := manufacturerPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrManufacturerNotFound)
		return
	}

	page, err := pageRequest(r.URL.Query())
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	filter, err := catalogFilter(r)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	found, err := h.hardwareModels.List(r.Context(), id, filter, page)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toPageResponse(found, toHardwareModelResponse))
}

// POST /api/manufacturers/{id}/models adds a model to the manufacturer.
func (h *Handler) createHardwareModel(w http.ResponseWriter, r *http.Request, user domain.User) {
	withName(h, w, r, manufacturerPathID, domain.ErrManufacturerNotFound, http.StatusCreated,
		func(id domain.ManufacturerID, name string) (hardwareModelResponse, error) {
			m, err := h.hardwareModels.Create(r.Context(), user.ID, id, name)
			return toHardwareModelResponse(m), err
		})
}

// GET /api/hardware-models/{id}. Deleted models are returned too.
func (h *Handler) getHardwareModel(w http.ResponseWriter, r *http.Request, _ domain.User) {
	id, ok := hardwareModelPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrHardwareModelNotFound)
		return
	}

	m, err := h.hardwareModels.Get(r.Context(), id)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toHardwareModelResponse(m))
}

// PATCH /api/hardware-models/{id} renames the model.
func (h *Handler) updateHardwareModel(w http.ResponseWriter, r *http.Request, user domain.User) {
	withName(h, w, r, hardwareModelPathID, domain.ErrHardwareModelNotFound, http.StatusOK,
		func(id domain.HardwareModelID, name string) (hardwareModelResponse, error) {
			m, err := h.hardwareModels.Rename(r.Context(), user.ID, id, name)
			return toHardwareModelResponse(m), err
		})
}

// DELETE /api/hardware-models/{id} marks the model deleted.
func (h *Handler) deleteHardwareModel(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := hardwareModelPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrHardwareModelNotFound)
		return
	}

	if err := h.hardwareModels.Delete(r.Context(), user.ID, id); err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/hardware-models/{id}/restore undoes a delete.
func (h *Handler) restoreHardwareModel(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := hardwareModelPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrHardwareModelNotFound)
		return
	}

	m, err := h.hardwareModels.Restore(r.Context(), user.ID, id)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toHardwareModelResponse(m))
}

// withName serves a request that names an entry by path ID and carries a
// catalogNameRequest body: it parses both, calls do and writes its result
// with status. An unparsable ID answers notFound.
func withName[ID, R any](
	h *Handler, w http.ResponseWriter, r *http.Request, pathID func(*http.Request) (ID, bool), notFound error, status int,
	do func(id ID, name string) (R, error),
) {
	id, ok := pathID(r)
	if !ok {
		h.writeDomainError(w, r, notFound)
		return
	}

	var req catalogNameRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	resp, err := do(id, req.Name)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, status, resp)
}

func manufacturerPathID(r *http.Request) (domain.ManufacturerID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))

	return domain.ManufacturerID(id), err == nil
}

func hardwareModelPathID(r *http.Request) (domain.HardwareModelID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))

	return domain.HardwareModelID(id), err == nil
}
