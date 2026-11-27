package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

type catalogEntryJSON struct {
	ID             string  `json:"id"`
	ManufacturerID string  `json:"manufacturerId"`
	Name           string  `json:"name"`
	ModelCount     int64   `json:"modelCount"`
	ItemCount      int64   `json:"itemCount"`
	DeletedAt      *string `json:"deletedAt"`
}

type hardwareBodyJSON struct {
	ManufacturerID *string `json:"manufacturerId"`
	ModelID        *string `json:"modelId"`
	SerialNumber   string  `json:"serialNumber"`
	Manufacturer   *struct {
		Name    string `json:"name"`
		Deleted bool   `json:"deleted"`
	} `json:"manufacturer"`
	Model *struct {
		Name    string `json:"name"`
		Deleted bool   `json:"deleted"`
	} `json:"model"`
}

func (e *testEnv) createCatalogEntry(t *testing.T, c *http.Client, path, name string) catalogEntryJSON {
	t.Helper()

	r := e.do(t, c, http.MethodPost, path, `{"name":"`+name+`"}`)
	wantStatus(t, r, http.StatusCreated)

	var entry catalogEntryJSON
	r.json(t, &entry)

	return entry
}

func TestManufacturers_Lifecycle(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)
	acme := env.createCatalogEntry(t, c, "/api/manufacturers", "ACME")
	env.createCatalogEntry(t, c, "/api/manufacturers", "Beta")

	r := env.do(t, c, http.MethodPost, "/api/manufacturers", `{"name":"acme"}`)
	if r.status != http.StatusConflict || r.errorCode(t) != "manufacturer_name_taken" {
		t.Fatalf("duplicate = %d %s", r.status, r.body)
	}

	s24 := env.createCatalogEntry(t, c, "/api/manufacturers/"+acme.ID+"/models", "S24")
	if s24.ManufacturerID != acme.ID {
		t.Fatalf("model = %+v", s24)
	}

	r = env.do(t, c, http.MethodGet, "/api/manufacturers?q=ac", "")
	wantStatus(t, r, http.StatusOK)

	var page pageJSON[catalogEntryJSON]
	r.json(t, &page)

	if len(page.Items) != 1 || page.Items[0].Name != "ACME" || page.Items[0].ModelCount != 1 {
		t.Fatalf("list = %+v", page)
	}

	r = env.do(t, c, http.MethodPatch, "/api/hardware-models/"+s24.ID, `{"name":"S24 Pro"}`)
	wantStatus(t, r, http.StatusOK)

	wantStatus(t, env.do(t, c, http.MethodDelete, "/api/manufacturers/"+acme.ID, ""), http.StatusNoContent)

	r = env.do(t, c, http.MethodPost, "/api/manufacturers/"+acme.ID+"/models", `{"name":"S48"}`)
	if r.status != http.StatusConflict || r.errorCode(t) != "manufacturer_deleted" {
		t.Fatalf("model of deleted manufacturer = %d %s", r.status, r.body)
	}

	r = env.do(t, c, http.MethodGet, "/api/manufacturers", "")
	r.json(t, &page)

	if len(page.Items) != 1 || page.Items[0].Name != "Beta" {
		t.Fatalf("live list = %+v", page)
	}

	wantStatus(t, env.do(t, c, http.MethodPost, "/api/manufacturers/"+acme.ID+"/restore", ""), http.StatusOK)

	r = env.do(t, c, http.MethodGet, "/api/manufacturers/"+acme.ID+"/models", "")
	wantStatus(t, r, http.StatusOK)
	r.json(t, &page)

	if len(page.Items) != 1 || page.Items[0].Name != "S24 Pro" {
		t.Fatalf("models = %+v", page)
	}

	wantStatus(t, env.do(t, c, http.MethodGet, "/api/manufacturers/nope", ""), http.StatusNotFound)
	wantStatus(t, env.do(t, c, http.MethodGet, "/api/hardware-models/nope", ""), http.StatusNotFound)
}

func TestStorageItems_HardwareRefs(t *testing.T) {
	t.Parallel()

	env, c, shelf, typ := itemEnv(t)
	acme := env.createCatalogEntry(t, c, "/api/manufacturers", "ACME")
	s24 := env.createCatalogEntry(t, c, "/api/manufacturers/"+acme.ID+"/models", "S24")

	item := env.createStorageItem(t, c, `{"assetTag":"HW-1","name":"Core","storageId":"`+shelf.ID+`","typeId":"`+typ.ID+
		`","usable":true,"modules":[{"kind":"hardware","hardware":{"manufacturerId":"`+acme.ID+`","modelId":"`+s24.ID+
		`","serialNumber":"SN-1"}}]}`)

	var hw hardwareBodyJSON
	if err := json.Unmarshal(item.Modules[0].Hardware, &hw); err != nil {
		t.Fatal(err)
	}

	if hw.ManufacturerID == nil || *hw.ManufacturerID != acme.ID || hw.Model == nil || hw.Model.Name != "S24" ||
		hw.Manufacturer == nil || hw.Manufacturer.Name != "ACME" || hw.SerialNumber != "SN-1" {
		t.Fatalf("hardware = %s", item.Modules[0].Hardware)
	}

	r := env.do(t, c, http.MethodGet, "/api/storage-items?q=s24", "")
	wantStatus(t, r, http.StatusOK)

	var page pageJSON[storageItemJSON]
	r.json(t, &page)

	if len(page.Items) != 1 || page.Items[0].ID != item.ID {
		t.Fatalf("search = %+v", page)
	}
}

func TestStorageItems_InvalidHardwareRefs(t *testing.T) {
	t.Parallel()

	env, c, shelf, typ := itemEnv(t)
	acme := env.createCatalogEntry(t, c, "/api/manufacturers", "ACME")
	s24 := env.createCatalogEntry(t, c, "/api/manufacturers/"+acme.ID+"/models", "S24")
	base := `{"assetTag":"HW-2","name":"x","storageId":"` + shelf.ID + `","typeId":"` + typ.ID + `","usable":true,"modules":[`

	for _, tt := range []struct{ name, hardware, field string }{
		{"bad id", `{"manufacturerId":"nope"}`, "modules[0].hardware.manufacturerId"},
		{"model without manufacturer", `{"modelId":"` + s24.ID + `"}`, "modules[0].hardware.manufacturerId"},
		{"unknown manufacturer", `{"manufacturerId":"` + s24.ID + `"}`, "modules[0].hardware.manufacturerId"},
	} {
		r := env.do(t, c, http.MethodPost, "/api/storage-items", base+`{"kind":"hardware","hardware":`+tt.hardware+`}]}`)
		if r.status != http.StatusUnprocessableEntity {
			t.Fatalf("%s = %d %s", tt.name, r.status, r.body)
		}

		var body struct {
			Error struct {
				Fields map[string]string `json:"fields"`
			} `json:"error"`
		}
		r.json(t, &body)

		if _, ok := body.Error.Fields[tt.field]; !ok {
			t.Fatalf("%s fields = %v, want %s", tt.name, body.Error.Fields, tt.field)
		}
	}
}
