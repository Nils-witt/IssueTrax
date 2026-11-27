package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

type ipConfigJSON struct {
	Mode         string   `json:"mode"`
	Address      *string  `json:"address"`
	PrefixLength *int     `json:"prefixLength"`
	Gateway      *string  `json:"gateway"`
	DNSServers   []string `json:"dnsServers"`
}

type networkInterfaceJSON struct {
	Name        string       `json:"name"`
	MAC         string       `json:"mac"`
	Type        string       `json:"type"`
	SSID        string       `json:"ssid"`
	NativeVLAN  *int         `json:"nativeVlan"`
	TaggedVLANs []int        `json:"taggedVlans"`
	IPv4        ipConfigJSON `json:"ipv4"`
	IPv6        ipConfigJSON `json:"ipv6"`
}

type networkDeviceJSON struct {
	ID                  string                 `json:"id"`
	StorageID           string                 `json:"storageId"`
	Name                string                 `json:"name"`
	Role                string                 `json:"role"`
	Hostname            string                 `json:"hostname"`
	PrimaryInterface    networkInterfaceJSON   `json:"primaryInterface"`
	SecondaryInterfaces []networkInterfaceJSON `json:"secondaryInterfaces"`
	DeletedAt           *string                `json:"deletedAt"`
}

const routerJSON = `{
	"storageId": "%s", "name": " Router ", "role": "router", "hostname": "gw.example.org",
	"primaryInterface": {
		"name": "eth0", "mac": "AA-BB-CC-00-11-22", "type": "wired", "nativeVlan": 10, "taggedVlans": [20, 30],
		"ipv4": {"mode": "static", "address": "192.168.1.10", "prefixLength": 24, "gateway": "192.168.1.1", "dnsServers": ["9.9.9.9"]},
		"ipv6": {"mode": "slaac"}
	},
	"secondaryInterfaces": [{
		"name": "wlan0", "mac": "02:00:00:00:00:01", "type": "wireless", "ssid": "Office",
		"ipv4": {"mode": "dhcp"}, "ipv6": {"mode": "off"}
	}]
}`

func (e *testEnv) createDevice(t *testing.T, c *http.Client, storageID string) networkDeviceJSON {
	t.Helper()

	r := e.do(t, c, http.MethodPost, "/api/network-devices", fmt.Sprintf(routerJSON, storageID))
	wantStatus(t, r, http.StatusCreated)

	var d networkDeviceJSON
	r.json(t, &d)

	return d
}

func TestNetworkDevices_RequireSession(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	id := "0190d2a4-7f3e-7c4b-9e3a-2b1c0d9e8f7a"

	for _, path := range []string{"/api/network-devices", "/api/network-devices/" + id, "/api/storages/" + id + "/network-devices"} {
		r := env.do(t, newClient(t), http.MethodGet, path, "")
		if r.status != http.StatusUnauthorized || r.errorCode(t) != "unauthenticated" {
			t.Errorf("GET %s = %d %s, want 401 unauthenticated", path, r.status, r.body)
		}
	}
}

// wantRouter fails unless d is the device routerJSON creates.
func wantRouter(t *testing.T, d networkDeviceJSON, storageID string) {
	t.Helper()

	str := func(s string) *string { return &s }
	num := func(n int) *int { return &n }

	want := networkDeviceJSON{
		ID: d.ID, StorageID: storageID, Name: "Router", Role: "router", Hostname: "gw.example.org",
		PrimaryInterface: networkInterfaceJSON{
			Name: "eth0", MAC: "aa:bb:cc:00:11:22", Type: "wired", NativeVLAN: num(10), TaggedVLANs: []int{20, 30},
			IPv4: ipConfigJSON{
				Mode: "static", Address: str("192.168.1.10"), PrefixLength: num(24), Gateway: str("192.168.1.1"),
				DNSServers: []string{"9.9.9.9"},
			},
			IPv6: ipConfigJSON{Mode: "slaac", DNSServers: []string{}},
		},
		SecondaryInterfaces: []networkInterfaceJSON{{
			Name: "wlan0", MAC: "02:00:00:00:00:01", Type: "wireless", SSID: "Office", TaggedVLANs: []int{},
			IPv4: ipConfigJSON{Mode: "dhcp", DNSServers: []string{}},
			IPv6: ipConfigJSON{Mode: "off", DNSServers: []string{}},
		}},
	}

	if got, wantJSON := mustJSON(t, d), mustJSON(t, want); got != wantJSON {
		t.Fatalf("device = %s\nwant     %s", got, wantJSON)
	}
}

func TestNetworkDevices_CreateAndGet(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)
	_, shelf := storageTree(t, env, c)

	d := env.createDevice(t, c, shelf.ID)
	wantRouter(t, d, shelf.ID)

	r := env.do(t, c, http.MethodGet, "/api/network-devices/"+d.ID, "")
	wantStatus(t, r, http.StatusOK)

	var got networkDeviceJSON
	r.json(t, &got)
	wantRouter(t, got, shelf.ID)

	// Generic item routes do not serve devices.
	if r := env.do(t, c, http.MethodGet, "/api/generic-storage-items/"+d.ID, ""); r.status != http.StatusNotFound {
		t.Fatalf("generic get of a device = %d", r.status)
	}
}

func TestNetworkDevices_ListAndSearch(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)
	root, shelf := storageTree(t, env, c)
	d := env.createDevice(t, c, shelf.ID)

	for path, want := range map[string]int64{
		"/api/storages/" + root.ID + "/network-devices?recursive=true": 1,
		"/api/storages/" + root.ID + "/network-devices":                0,
		"/api/network-devices?q=BB:CC":                                 1,
		"/api/network-devices?q=gw.example":                            1,
		"/api/network-devices?q=nothing":                               0,
	} {
		r := env.do(t, c, http.MethodGet, path, "")
		wantStatus(t, r, http.StatusOK)

		var list pageJSON[networkDeviceJSON]
		r.json(t, &list)

		if list.Total != want || (want == 1 && list.Items[0].ID != d.ID) {
			t.Errorf("GET %s = %s, want %d", path, r.body, want)
		}
	}
}

func TestNetworkDevices_SwapPrimaryDeleteAndRestore(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)
	_, shelf := storageTree(t, env, c)
	d := env.createDevice(t, c, shelf.ID)

	// Make the wireless interface primary.
	swap := map[string]any{
		"primaryInterface":    d.SecondaryInterfaces[0],
		"secondaryInterfaces": []networkInterfaceJSON{d.PrimaryInterface},
		"role":                "switch",
	}

	r := env.do(t, c, http.MethodPatch, "/api/network-devices/"+d.ID, mustJSON(t, swap))
	wantStatus(t, r, http.StatusOK)

	var swapped networkDeviceJSON
	r.json(t, &swapped)

	if swapped.PrimaryInterface.Name != "wlan0" || swapped.SecondaryInterfaces[0].Name != "eth0" || swapped.Role != "switch" ||
		*swapped.SecondaryInterfaces[0].IPv4.Address != "192.168.1.10" {
		t.Fatalf("swapped = %s", r.body)
	}

	wantStatus(t, env.do(t, c, http.MethodDelete, "/api/network-devices/"+d.ID, ""), http.StatusNoContent)

	r = env.do(t, c, http.MethodPatch, "/api/network-devices/"+d.ID, `{"name":"x"}`)
	if r.status != http.StatusConflict || r.errorCode(t) != "storage_item_deleted" {
		t.Fatalf("patch deleted = %d %s", r.status, r.body)
	}

	wantStatus(t, env.do(t, c, http.MethodPost, "/api/network-devices/"+d.ID+"/restore", ""), http.StatusOK)
}

func TestNetworkDevices_Validation(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)
	_, shelf := storageTree(t, env, c)

	r := env.do(t, c, http.MethodPost, "/api/network-devices", `{
		"storageId": "`+shelf.ID+`", "name": "Router", "role": "hub",
		"primaryInterface": {
			"name": "eth0", "mac": "nope", "type": "wired",
			"ipv4": {"mode": "static", "address": "10.0.0.2", "prefixLength": 24, "gateway": "10.0.1.1", "dnsServers": ["2001:db8::1"]},
			"ipv6": {"mode": "dhcp"}
		},
		"secondaryInterfaces": [{"name": "", "mac": "02:00:00:00:00:01", "type": "wired", "ipv4": {"mode": "off"}, "ipv6": {"mode": "off"}}]
	}`)
	wantStatus(t, r, http.StatusUnprocessableEntity)

	var body struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	r.json(t, &body)

	want := map[string]string{
		"role":                                "invalid",
		"primaryInterface.mac":                "invalid",
		"primaryInterface.ipv4.gateway":       "invalid",
		"primaryInterface.ipv4.dnsServers[0]": "invalid",
		"secondaryInterfaces[0].name":         "required",
	}

	for field, code := range want {
		if body.Error.Fields[field] != code {
			t.Errorf("fields[%s] = %q, want %q (all: %v)", field, body.Error.Fields[field], code, body.Error.Fields)
		}
	}

	r = env.do(t, c, http.MethodPost, "/api/network-devices", `{"storageId":"`+shelf.ID+`","unknown":1}`)
	wantStatus(t, r, http.StatusBadRequest)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}
