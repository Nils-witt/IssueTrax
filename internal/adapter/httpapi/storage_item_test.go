package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

type userRefJSON struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type moduleBodyJSON struct {
	ID              string          `json:"id"`
	Kind            string          `json:"kind"`
	Primary         bool            `json:"primary"`
	Hardware        json.RawMessage `json:"hardware"`
	Purchase        *purchaseJSON   `json:"purchase"`
	SimCard         *simCardJSON    `json:"simCard"`
	WWAN            *wwanJSON       `json:"wwan"`
	SIPClient       *sipClientJSON  `json:"sipClient"`
	Bluetooth       *bluetoothJSON  `json:"bluetooth"`
	OperatingSystem *struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"operatingSystem"`
	NetworkInterface *struct {
		Name string `json:"name"`
		MAC  string `json:"mac"`
	} `json:"networkInterface"`
}

type simCardJSON struct {
	Provider    string `json:"provider"`
	PhoneNumber string `json:"phoneNumber"`
	ICCID       string `json:"iccid"`
	IMSI        string `json:"imsi"`
}

type wwanJSON struct {
	IMEI          string  `json:"imei"`
	APN           string  `json:"apn"`
	Technology    string  `json:"technology"`
	SimCardItemID *string `json:"simCardItemId"`
	SimCardItem   *struct {
		AssetTag string `json:"assetTag"`
		Name     string `json:"name"`
	} `json:"simCardItem"`
}

type sipClientJSON struct {
	Server    string `json:"server"`
	Port      int    `json:"port"`
	Transport string `json:"transport"`
	Username  string `json:"username"`
	Extension string `json:"extension"`
}

type bluetoothJSON struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Version string `json:"version"`
}

type purchaseJSON struct {
	Price    *string `json:"price"`
	Currency string  `json:"currency"`
}

type storageItemJSON struct {
	ID                string           `json:"id"`
	AssetTag          string           `json:"assetTag"`
	Name              string           `json:"name"`
	StorageID         string           `json:"storageId"`
	Type              itemTypeRefJSON  `json:"type"`
	Usable            bool             `json:"usable"`
	DecommissionedAt  *string          `json:"decommissionedAt"`
	PersonResponsible *userRefJSON     `json:"personResponsible"`
	Modules           []moduleBodyJSON `json:"modules"`
	DeletedAt         *string          `json:"deletedAt"`
}

const ethJSON = `{"name":"eth0","mac":"AA-BB-CC-00-11-22","type":"wired","ipv4":{"mode":"dhcp"},"ipv6":{"mode":"off"}}`

func (e *testEnv) createStorageItem(t *testing.T, c *http.Client, body string) storageItemJSON {
	t.Helper()

	r := e.do(t, c, http.MethodPost, "/api/storage-items", body)
	wantStatus(t, r, http.StatusCreated)

	var item storageItemJSON
	r.json(t, &item)

	return item
}

// itemEnv is an admin session with a storage tree and a Switch type.
func itemEnv(t *testing.T) (env *testEnv, c *http.Client, shelf storageJSON, typ itemTypeJSON) {
	t.Helper()

	env, c = signedInAdmin(t)
	_, shelf = storageTree(t, env, c)
	typ = env.createItemType(t, c, `{"name":"Switch","tagPrefix":"NET-"}`)

	return env, c, shelf, typ
}

func TestStorageItems_CreateGetUpdate(t *testing.T) {
	t.Parallel()

	env, c, shelf, typ := itemEnv(t)
	me := env.me(t, c)

	item := env.createStorageItem(t, c, `{"assetTag":"NET-00001","name":"Core","storageId":"`+shelf.ID+`","typeId":"`+typ.ID+
		`","usable":true,"personResponsibleId":"`+me+`","modules":[`+
		`{"kind":"network_interface","primary":true,"networkInterface":`+ethJSON+`},`+
		`{"kind":"purchase","purchase":{"price":"12.5","currency":"eur","supplier":"Shop"}},`+
		`{"kind":"sim_card","simCard":{"provider":"Telekom","phoneNumber":"+49 171 1234567","iccid":"8949 0200 0012 3456 7890","imsi":"262 01 1234567890"}},`+
		`{"kind":"wwan","wwan":{"imei":"49-015420-323751-8","apn":"internet.telekom","technology":"5g"}},`+
		`{"kind":"operating_system","operatingSystem":{"name":" Debian ","version":"12"}},`+
		`{"kind":"sip_client","sipClient":{"server":" PBX.example.org ","port":5061,"transport":"tls","username":"ada","extension":"10 42"}},`+
		`{"kind":"bluetooth","bluetooth":{"name":" Core BT ","address":"CC-DD-EE-00-11-33","version":"5.3"}},`+
		`{"kind":"hardware","hardware":{"serialNumber":"S24"}}]}`)

	wantCreatedItem(t, item, me)

	eth := item.Modules[7]

	r := env.do(t, c, http.MethodPatch, "/api/storage-items/"+item.ID, `{"usable":false,"personResponsibleId":null,`+
		`"decommissionedAt":"2026-05-01T00:00:00Z","decommissioningReason":"broken","modules":[`+
		`{"id":"`+eth.ID+`","kind":"network_interface","networkInterface":`+ethJSON+`}]}`)
	wantStatus(t, r, http.StatusOK)

	var updated storageItemJSON
	r.json(t, &updated)

	if updated.Usable || updated.PersonResponsible != nil || updated.DecommissionedAt == nil || len(updated.Modules) != 1 ||
		updated.Modules[0].ID != eth.ID || updated.Modules[0].Primary {
		t.Fatalf("updated = %+v", updated)
	}

	r = env.do(t, c, http.MethodPatch, "/api/storage-items/"+item.ID, `{"assetTag":"X"}`)
	if r.status != http.StatusUnprocessableEntity {
		t.Fatalf("asset tag change = %d %s", r.status, r.body)
	}

	wantStatus(t, env.do(t, c, http.MethodDelete, "/api/storage-items/"+item.ID, ""), http.StatusNoContent)

	r = env.do(t, c, http.MethodPost, "/api/storage-items/"+item.ID+"/restore", "")
	wantStatus(t, r, http.StatusOK)
}

// wantCreatedItem checks the item TestStorageItems_CreateGetUpdate creates.
func wantCreatedItem(t *testing.T, item storageItemJSON, me string) {
	t.Helper()

	if item.AssetTag != "NET-00001" || item.Type.Name != "Switch" || item.PersonResponsible == nil ||
		item.PersonResponsible.ID != me || len(item.Modules) != 8 || item.Modules[0].Kind != "hardware" {
		t.Fatalf("created = %+v", item)
	}

	if p := item.Modules[1].Purchase; p == nil || *p.Price != "12.50" || p.Currency != "EUR" ||
		item.Modules[7].NetworkInterface.MAC != "aa:bb:cc:00:11:22" {
		t.Fatalf("created modules = %+v", item.Modules)
	}

	wantCreatedPayloads(t, item)
}

// wantCreatedPayloads checks the SIM card, WWAN and operating system
// payloads (and, via wantCreatedDevicePayloads, the rest) of the item created in
// TestStorageItems_CreateGetUpdate.
func wantCreatedPayloads(t *testing.T, item storageItemJSON) {
	t.Helper()

	if sc := item.Modules[2].SimCard; sc == nil || *sc != (simCardJSON{
		Provider: "Telekom", PhoneNumber: "+491711234567", ICCID: "89490200001234567890", IMSI: "262011234567890",
	}) {
		t.Fatalf("created sim card = %+v", item.Modules[2])
	}

	if w := item.Modules[3].WWAN; w == nil || w.IMEI != "490154203237518" || w.APN != "internet.telekom" || w.Technology != "5g" || w.SimCardItemID != nil {
		t.Fatalf("created wwan = %+v", item.Modules[3])
	}

	if sys := item.Modules[4].OperatingSystem; sys == nil || sys.Name != "Debian" || sys.Version != "12" {
		t.Fatalf("created operating system = %+v", item.Modules[4])
	}

	wantCreatedDevicePayloads(t, item)
}

// wantCreatedDevicePayloads checks the SIP client and Bluetooth payloads of
// the item created in TestStorageItems_CreateGetUpdate.
func wantCreatedDevicePayloads(t *testing.T, item storageItemJSON) {
	t.Helper()

	if sip := item.Modules[5].SIPClient; sip == nil || *sip != (sipClientJSON{
		Server: "PBX.example.org", Port: 5061, Transport: "tls", Username: "ada", Extension: "1042",
	}) {
		t.Fatalf("created sip client = %+v", item.Modules[5])
	}

	if bt := item.Modules[6].Bluetooth; bt == nil || *bt != (bluetoothJSON{
		Name: "Core BT", Address: "cc:dd:ee:00:11:33", Version: "5.3",
	}) {
		t.Fatalf("created bluetooth = %+v", item.Modules[6])
	}
}

func TestStorageItems_ListAndFilters(t *testing.T) {
	t.Parallel()

	env, c, shelf, typ := itemEnv(t)

	env.createStorageItem(t, c, `{"assetTag":"A-1","name":"Router","storageId":"`+shelf.ID+`","typeId":"`+typ.ID+`","usable":true,`+
		`"modules":[{"kind":"network_interface","networkInterface":`+ethJSON+`}]}`)
	env.createStorageItem(t, c, `{"assetTag":"A-2","name":"Chair","storageId":"`+shelf.ID+`","typeId":"`+typ.ID+`","usable":false,`+
		`"decommissionedAt":"2026-01-01T00:00:00Z"}`)

	list := func(path string) []string {
		t.Helper()

		r := env.do(t, c, http.MethodGet, path, "")
		wantStatus(t, r, http.StatusOK)

		var page pageJSON[storageItemJSON]
		r.json(t, &page)

		names := make([]string, 0, len(page.Items))
		for _, i := range page.Items {
			names = append(names, i.Name)
		}

		return names
	}

	for path, want := range map[string]int{
		"/api/storages/" + shelf.ID + "/storage-items":                      2,
		"/api/storage-items?usable=false":                                   1,
		"/api/storage-items?decommissioned=false":                           1,
		"/api/storage-items?typeId=" + typ.ID + "&includeSubtypes=true":     2,
		"/api/storage-items?q=aa:bb:cc:00:11:22":                            1,
		"/api/storage-items?q=a-2":                                          1,
		"/api/storage-items?deleted=true":                                   0,
		"/api/storages/" + shelf.ID + "/storage-items?recursive=true&q=rou": 1,
	} {
		if got := list(path); len(got) != want {
			t.Errorf("GET %s = %v, want %d items", path, got, want)
		}
	}

	for _, path := range []string{
		"/api/storage-items?usable=maybe", "/api/storage-items?typeId=nope", "/api/storage-items?responsibleId=nope",
	} {
		wantStatus(t, env.do(t, c, http.MethodGet, path, ""), http.StatusUnprocessableEntity)
	}
}

func TestStorageItems_Errors(t *testing.T) {
	t.Parallel()

	env, c, shelf, typ := itemEnv(t)
	env.createStorageItem(t, c, `{"assetTag":"A-1","name":"Router","storageId":"`+shelf.ID+`","typeId":"`+typ.ID+`","usable":true}`)

	valid := `"storageId":"` + shelf.ID + `","typeId":"` + typ.ID + `"`

	tests := []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"taken asset tag", http.MethodPost, "/api/storage-items", `{"assetTag":"a-1","name":"x","usable":true,` + valid + `}`, http.StatusConflict, "asset_tag_taken"},
		{"missing usable", http.MethodPost, "/api/storage-items", `{"assetTag":"B","name":"x",` + valid + `}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"bad type id", http.MethodPost, "/api/storage-items", `{"assetTag":"B","name":"x","usable":true,"storageId":"` + shelf.ID + `","typeId":"nope"}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"bad module id", http.MethodPost, "/api/storage-items", `{"assetTag":"B","name":"x","usable":true,` + valid + `,"modules":[{"id":"nope","kind":"hardware","hardware":{}}]}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"unknown item", http.MethodGet, "/api/storage-items/0190d2a4-7f3e-7c4b-9e3a-2b1c0d9e8f7a", "", http.StatusNotFound, "storage_item_not_found"},
		{"unknown storage", http.MethodGet, "/api/storages/0190d2a4-7f3e-7c4b-9e3a-2b1c0d9e8f7a/storage-items", "", http.StatusNotFound, "storage_not_found"},
	}

	for _, tt := range tests {
		r := env.do(t, c, tt.method, tt.path, tt.body)
		if r.status != tt.status || r.errorCode(t) != tt.code {
			t.Errorf("%s: %d %s, want %d %s", tt.name, r.status, r.body, tt.status, tt.code)
		}
	}
}

func TestStorageItems_WWANSimCardItem(t *testing.T) {
	t.Parallel()

	env, c, shelf, typ := itemEnv(t)
	base := `"storageId":"` + shelf.ID + `","typeId":"` + typ.ID + `","usable":true`

	sim := env.createStorageItem(t, c, `{"assetTag":"SIM-1","name":"SIM 1",`+base+
		`,"modules":[{"kind":"sim_card","simCard":{"provider":"Telekom","phoneNumber":"","iccid":"","imsi":""}}]}`)

	router := env.createStorageItem(t, c, `{"assetTag":"R-1","name":"LTE router",`+base+
		`,"modules":[{"kind":"wwan","wwan":{"imei":"","apn":"internet","technology":"4g","simCardItemId":"`+sim.ID+`"}}]}`)

	if w := router.Modules[0].WWAN; w == nil || w.SimCardItemID == nil || *w.SimCardItemID != sim.ID ||
		w.SimCardItem == nil || w.SimCardItem.AssetTag != "SIM-1" || w.SimCardItem.Name != "SIM 1" {
		t.Fatalf("wwan = %+v", router.Modules[0].WWAN)
	}

	r := env.do(t, c, http.MethodPost, "/api/storage-items", `{"assetTag":"R-2","name":"Other",`+base+
		`,"modules":[{"kind":"wwan","wwan":{"imei":"","apn":"","technology":"","simCardItemId":"`+router.ID+`"}}]}`)
	if r.status != http.StatusUnprocessableEntity {
		t.Fatalf("item without sim card = %d %s", r.status, r.body)
	}
}
