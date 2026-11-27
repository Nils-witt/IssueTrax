-- name: CreateStorageItem :exec
INSERT INTO storage_items (
    id, asset_tag, storage_id, item_type_id, name, description, usable, order_date, commissioned_at,
    decommissioned_at, decommissioning_reason, responsible_id, created_at, updated_at, deleted_at
) VALUES (
    @id, @asset_tag, @storage_id, @item_type_id, @name, @description, @usable, @order_date, @commissioned_at,
    @decommissioned_at, @decommissioning_reason, @responsible_id, @created_at, @updated_at, @deleted_at
);

-- name: UpdateStorageItem :execrows
-- The asset tag never changes.
UPDATE storage_items
SET storage_id = @storage_id, item_type_id = @item_type_id, name = @name, description = @description,
    usable = @usable, order_date = @order_date, commissioned_at = @commissioned_at,
    decommissioned_at = @decommissioned_at, decommissioning_reason = @decommissioning_reason,
    responsible_id = @responsible_id, updated_at = @updated_at, deleted_at = @deleted_at
WHERE id = @id;

-- name: GetStorageItemsByIDs :many
-- Items, deleted ones included.
SELECT * FROM storage_items WHERE id = ANY (@ids::uuid[]);

-- name: StorageItemAssetTagTaken :one
SELECT EXISTS (SELECT 1 FROM storage_items WHERE lower(asset_tag) = lower(@asset_tag::text));

-- name: ListStorageItems :many
-- One page of the items matching the filters, ordered by name. A NULL
-- @storage_id searches all storages; @recursive adds its subtree. A NULL
-- @type_id keeps every type; @include_subtypes adds its descendants.
-- @pattern is a pre-escaped ILIKE pattern ('%term%'), empty for no search;
-- @mac matches interface MACs and Bluetooth addresses exactly.
WITH RECURSIVE subtree (id) AS (
    SELECT sqlc.narg('storage_id')::uuid WHERE sqlc.narg('storage_id')::uuid IS NOT NULL
    UNION
    SELECT s.id FROM storages s JOIN subtree t ON s.parent_id = t.id WHERE @recursive::boolean
), types (id) AS (
    SELECT sqlc.narg('type_id')::uuid WHERE sqlc.narg('type_id')::uuid IS NOT NULL
    UNION
    SELECT it.id FROM item_types it JOIN types p ON it.parent_id = p.id WHERE @include_subtypes::boolean
)
SELECT i.* FROM storage_items i
WHERE (sqlc.narg('storage_id')::uuid IS NULL OR i.storage_id IN (SELECT id FROM subtree))
  AND (sqlc.narg('type_id')::uuid IS NULL OR i.item_type_id IN (SELECT id FROM types))
  AND (i.deleted_at IS NOT NULL) = @deleted::boolean
  AND (sqlc.narg('usable')::boolean IS NULL OR i.usable = sqlc.narg('usable')::boolean)
  AND (sqlc.narg('responsible_id')::uuid IS NULL OR i.responsible_id = sqlc.narg('responsible_id')::uuid)
  AND (sqlc.narg('decommissioned')::boolean IS NULL
       OR (i.decommissioned_at IS NOT NULL) = sqlc.narg('decommissioned')::boolean)
  AND (@pattern::text = ''
       OR i.name ILIKE @pattern::text OR i.description ILIKE @pattern::text OR i.asset_tag ILIKE @pattern::text
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_hardware h ON h.module_id = m.id
                      LEFT JOIN manufacturers mf ON mf.id = h.manufacturer_id
                      LEFT JOIN hardware_models hm ON hm.id = h.model_id
                  WHERE m.item_id = i.id AND (mf.name ILIKE @pattern::text OR hm.name ILIKE @pattern::text
                                              OR h.serial_number ILIKE @pattern::text))
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_network_hosts nh ON nh.module_id = m.id
                  WHERE m.item_id = i.id AND nh.hostname ILIKE @pattern::text)
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_sim_cards sc ON sc.module_id = m.id
                  WHERE m.item_id = i.id AND (sc.provider ILIKE @pattern::text OR sc.phone_number ILIKE @pattern::text
                                              OR sc.iccid ILIKE @pattern::text OR sc.imsi ILIKE @pattern::text))
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_wwans w ON w.module_id = m.id
                  WHERE m.item_id = i.id AND (w.imei ILIKE @pattern::text OR w.apn ILIKE @pattern::text))
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_operating_systems os ON os.module_id = m.id
                  WHERE m.item_id = i.id AND (os.name ILIKE @pattern::text OR os.version ILIKE @pattern::text))
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_sip_clients sip ON sip.module_id = m.id
                  WHERE m.item_id = i.id AND (sip.server ILIKE @pattern::text OR sip.username ILIKE @pattern::text
                                              OR sip.extension ILIKE @pattern::text))
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_bluetooth bt ON bt.module_id = m.id
                  WHERE m.item_id = i.id AND (bt.name ILIKE @pattern::text OR bt.address ILIKE @pattern::text
                                              OR bt.address = sqlc.narg('mac')::macaddr::text))
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_network_interfaces n ON n.module_id = m.id
                  WHERE m.item_id = i.id AND (n.mac::text ILIKE @pattern::text OR n.mac = sqlc.narg('mac')::macaddr)))
  AND (NOT @has_cursor::boolean OR (lower(i.name), i.id) > (@cursor_name::text, @cursor_id::uuid))
ORDER BY lower(i.name), i.id
LIMIT @row_limit;

-- name: CountStorageItems :one
-- Counts what ListStorageItems lists across all pages.
WITH RECURSIVE subtree (id) AS (
    SELECT sqlc.narg('storage_id')::uuid WHERE sqlc.narg('storage_id')::uuid IS NOT NULL
    UNION
    SELECT s.id FROM storages s JOIN subtree t ON s.parent_id = t.id WHERE @recursive::boolean
), types (id) AS (
    SELECT sqlc.narg('type_id')::uuid WHERE sqlc.narg('type_id')::uuid IS NOT NULL
    UNION
    SELECT it.id FROM item_types it JOIN types p ON it.parent_id = p.id WHERE @include_subtypes::boolean
)
SELECT count(*) FROM storage_items i
WHERE (sqlc.narg('storage_id')::uuid IS NULL OR i.storage_id IN (SELECT id FROM subtree))
  AND (sqlc.narg('type_id')::uuid IS NULL OR i.item_type_id IN (SELECT id FROM types))
  AND (i.deleted_at IS NOT NULL) = @deleted::boolean
  AND (sqlc.narg('usable')::boolean IS NULL OR i.usable = sqlc.narg('usable')::boolean)
  AND (sqlc.narg('responsible_id')::uuid IS NULL OR i.responsible_id = sqlc.narg('responsible_id')::uuid)
  AND (sqlc.narg('decommissioned')::boolean IS NULL
       OR (i.decommissioned_at IS NOT NULL) = sqlc.narg('decommissioned')::boolean)
  AND (@pattern::text = ''
       OR i.name ILIKE @pattern::text OR i.description ILIKE @pattern::text OR i.asset_tag ILIKE @pattern::text
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_hardware h ON h.module_id = m.id
                      LEFT JOIN manufacturers mf ON mf.id = h.manufacturer_id
                      LEFT JOIN hardware_models hm ON hm.id = h.model_id
                  WHERE m.item_id = i.id AND (mf.name ILIKE @pattern::text OR hm.name ILIKE @pattern::text
                                              OR h.serial_number ILIKE @pattern::text))
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_network_hosts nh ON nh.module_id = m.id
                  WHERE m.item_id = i.id AND nh.hostname ILIKE @pattern::text)
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_sim_cards sc ON sc.module_id = m.id
                  WHERE m.item_id = i.id AND (sc.provider ILIKE @pattern::text OR sc.phone_number ILIKE @pattern::text
                                              OR sc.iccid ILIKE @pattern::text OR sc.imsi ILIKE @pattern::text))
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_wwans w ON w.module_id = m.id
                  WHERE m.item_id = i.id AND (w.imei ILIKE @pattern::text OR w.apn ILIKE @pattern::text))
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_operating_systems os ON os.module_id = m.id
                  WHERE m.item_id = i.id AND (os.name ILIKE @pattern::text OR os.version ILIKE @pattern::text))
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_sip_clients sip ON sip.module_id = m.id
                  WHERE m.item_id = i.id AND (sip.server ILIKE @pattern::text OR sip.username ILIKE @pattern::text
                                              OR sip.extension ILIKE @pattern::text))
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_bluetooth bt ON bt.module_id = m.id
                  WHERE m.item_id = i.id AND (bt.name ILIKE @pattern::text OR bt.address ILIKE @pattern::text
                                              OR bt.address = sqlc.narg('mac')::macaddr::text))
       OR EXISTS (SELECT 1 FROM storage_item_modules m JOIN module_network_interfaces n ON n.module_id = m.id
                  WHERE m.item_id = i.id AND (n.mac::text ILIKE @pattern::text OR n.mac = sqlc.narg('mac')::macaddr)));

-- name: DeleteStorageItemModulesExcept :exec
-- Removes the item's modules whose ID is not in @keep_ids.
DELETE FROM storage_item_modules WHERE item_id = @item_id AND NOT (id = ANY (@keep_ids::uuid[]));

-- name: ClearStorageItemPrimaries :exec
UPDATE storage_item_modules SET is_primary = false WHERE item_id = @item_id AND is_primary;

-- name: UpsertStorageItemModule :execrows
-- Inserts a module instance or updates its primary flag; an instance of
-- another item or kind is left alone (0 rows).
INSERT INTO storage_item_modules (id, item_id, module, is_primary) VALUES (@id, @item_id, @module, @is_primary)
ON CONFLICT (id) DO UPDATE SET is_primary = EXCLUDED.is_primary
WHERE storage_item_modules.item_id = EXCLUDED.item_id AND storage_item_modules.module = EXCLUDED.module;

-- name: ListStorageItemModules :many
SELECT id, item_id, module, is_primary FROM storage_item_modules WHERE item_id = ANY (@item_ids::uuid[]);

-- name: UpsertHardwareModule :exec
INSERT INTO module_hardware (module_id, manufacturer_id, model_id, serial_number)
VALUES (@module_id, sqlc.narg('manufacturer_id'), sqlc.narg('model_id'), @serial_number)
ON CONFLICT (module_id) DO UPDATE
SET manufacturer_id = EXCLUDED.manufacturer_id, model_id = EXCLUDED.model_id, serial_number = EXCLUDED.serial_number;

-- name: ListHardwareModules :many
SELECT * FROM module_hardware WHERE module_id = ANY (@module_ids::uuid[]);

-- name: UpsertNetworkHostModule :exec
INSERT INTO module_network_hosts (module_id, role, hostname) VALUES (@module_id, @role, @hostname)
ON CONFLICT (module_id) DO UPDATE SET role = EXCLUDED.role, hostname = EXCLUDED.hostname;

-- name: ListNetworkHostModules :many
SELECT * FROM module_network_hosts WHERE module_id = ANY (@module_ids::uuid[]);

-- name: UpsertPurchaseModule :exec
INSERT INTO module_purchases (module_id, supplier, invoice_number, price_cents, currency, warranty_end)
VALUES (@module_id, @supplier, @invoice_number, @price_cents, @currency, @warranty_end)
ON CONFLICT (module_id) DO UPDATE
SET supplier = EXCLUDED.supplier, invoice_number = EXCLUDED.invoice_number, price_cents = EXCLUDED.price_cents,
    currency = EXCLUDED.currency, warranty_end = EXCLUDED.warranty_end;

-- name: ListPurchaseModules :many
SELECT * FROM module_purchases WHERE module_id = ANY (@module_ids::uuid[]);

-- name: UpsertSimCardModule :exec
INSERT INTO module_sim_cards (module_id, provider, phone_number, iccid, imsi)
VALUES (@module_id, @provider, @phone_number, @iccid, @imsi)
ON CONFLICT (module_id) DO UPDATE
SET provider = EXCLUDED.provider, phone_number = EXCLUDED.phone_number, iccid = EXCLUDED.iccid,
    imsi = EXCLUDED.imsi;

-- name: ListSimCardModules :many
SELECT * FROM module_sim_cards WHERE module_id = ANY (@module_ids::uuid[]);

-- name: UpsertWwanModule :exec
INSERT INTO module_wwans (module_id, imei, apn, technology, sim_card_item_id)
VALUES (@module_id, @imei, @apn, @technology, sqlc.narg('sim_card_item_id'))
ON CONFLICT (module_id) DO UPDATE
SET imei = EXCLUDED.imei, apn = EXCLUDED.apn, technology = EXCLUDED.technology,
    sim_card_item_id = EXCLUDED.sim_card_item_id;

-- name: ListWwanModules :many
SELECT * FROM module_wwans WHERE module_id = ANY (@module_ids::uuid[]);

-- name: UpsertOperatingSystemModule :exec
INSERT INTO module_operating_systems (module_id, name, version)
VALUES (@module_id, @name, @version)
ON CONFLICT (module_id) DO UPDATE
SET name = EXCLUDED.name, version = EXCLUDED.version;

-- name: ListOperatingSystemModules :many
SELECT * FROM module_operating_systems WHERE module_id = ANY (@module_ids::uuid[]);

-- name: UpsertSipClientModule :exec
INSERT INTO module_sip_clients (module_id, server, port, transport, username, extension)
VALUES (@module_id, @server, @port, @transport, @username, @extension)
ON CONFLICT (module_id) DO UPDATE
SET server = EXCLUDED.server, port = EXCLUDED.port, transport = EXCLUDED.transport,
    username = EXCLUDED.username, extension = EXCLUDED.extension;

-- name: ListSipClientModules :many
SELECT * FROM module_sip_clients WHERE module_id = ANY (@module_ids::uuid[]);

-- name: UpsertBluetoothModule :exec
INSERT INTO module_bluetooth (module_id, name, address, version)
VALUES (@module_id, @name, @address, @version)
ON CONFLICT (module_id) DO UPDATE
SET name = EXCLUDED.name, address = EXCLUDED.address, version = EXCLUDED.version;

-- name: ListBluetoothModules :many
SELECT * FROM module_bluetooth WHERE module_id = ANY (@module_ids::uuid[]);

-- name: UpsertNetworkInterfaceModule :exec
INSERT INTO module_network_interfaces (
    module_id, name, mac, type, ssid, native_vlan, tagged_vlans,
    ipv4_mode, ipv4_address, ipv4_gateway, ipv4_dns,
    ipv6_mode, ipv6_address, ipv6_gateway, ipv6_dns
) VALUES (
    @module_id, @name, @mac, @type, @ssid, @native_vlan, @tagged_vlans,
    @ipv4_mode, @ipv4_address, @ipv4_gateway, @ipv4_dns,
    @ipv6_mode, @ipv6_address, @ipv6_gateway, @ipv6_dns
)
ON CONFLICT (module_id) DO UPDATE
SET name = EXCLUDED.name, mac = EXCLUDED.mac, type = EXCLUDED.type, ssid = EXCLUDED.ssid,
    native_vlan = EXCLUDED.native_vlan, tagged_vlans = EXCLUDED.tagged_vlans,
    ipv4_mode = EXCLUDED.ipv4_mode, ipv4_address = EXCLUDED.ipv4_address, ipv4_gateway = EXCLUDED.ipv4_gateway,
    ipv4_dns = EXCLUDED.ipv4_dns, ipv6_mode = EXCLUDED.ipv6_mode, ipv6_address = EXCLUDED.ipv6_address,
    ipv6_gateway = EXCLUDED.ipv6_gateway, ipv6_dns = EXCLUDED.ipv6_dns;

-- name: ListNetworkInterfaceModules :many
SELECT * FROM module_network_interfaces WHERE module_id = ANY (@module_ids::uuid[]);
