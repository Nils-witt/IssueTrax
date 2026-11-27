-- The shared columns of network devices live in storage_items and use the
-- queries in storage_items.sql with kind 'network_device'.

-- name: CreateNetworkDevice :exec
INSERT INTO network_devices (item_id, role, hostname, manufacturer, model, serial_number)
VALUES (@item_id, @role, @hostname, @manufacturer, @model, @serial_number);

-- name: UpdateNetworkDevice :exec
UPDATE network_devices
SET role = @role, hostname = @hostname, manufacturer = @manufacturer, model = @model,
    serial_number = @serial_number
WHERE item_id = @item_id;

-- name: ListNetworkDevices :many
SELECT item_id, role, hostname, manufacturer, model, serial_number
FROM network_devices
WHERE item_id = ANY(@item_ids::uuid[]);

-- name: DeleteNetworkInterfaces :exec
DELETE FROM network_interfaces WHERE item_id = @item_id;

-- name: CreateNetworkInterface :exec
INSERT INTO network_interfaces (
    item_id, position, name, mac, type, ssid, native_vlan, tagged_vlans,
    ipv4_mode, ipv4_address, ipv4_gateway, ipv4_dns,
    ipv6_mode, ipv6_address, ipv6_gateway, ipv6_dns
) VALUES (
    @item_id, @position, @name, @mac, @type, @ssid, @native_vlan, @tagged_vlans,
    @ipv4_mode, @ipv4_address, @ipv4_gateway, @ipv4_dns,
    @ipv6_mode, @ipv6_address, @ipv6_gateway, @ipv6_dns
);

-- name: ListNetworkInterfaces :many
-- The interfaces of the devices in @item_ids, primary first.
SELECT item_id, position, name, mac, type, ssid, native_vlan, tagged_vlans,
       ipv4_mode, ipv4_address, ipv4_gateway, ipv4_dns,
       ipv6_mode, ipv6_address, ipv6_gateway, ipv6_dns
FROM network_interfaces
WHERE item_id = ANY(@item_ids::uuid[])
ORDER BY item_id, position;

-- name: SearchNetworkDevices :many
-- Like SearchStorageItems, but @pattern also matches the hostname, the
-- serial number and the MAC of any interface.
SELECT i.id, i.storage_id, i.kind, i.name, i.description, i.created_at, i.updated_at, i.deleted_at
FROM storage_items i
JOIN network_devices d ON d.item_id = i.id
WHERE i.kind = 'network_device'
  AND (i.deleted_at IS NOT NULL) = @deleted::boolean
  AND (@pattern::text = '' OR i.name ILIKE @pattern::text OR i.description ILIKE @pattern::text
       OR d.hostname ILIKE @pattern::text OR d.serial_number ILIKE @pattern::text
       OR EXISTS (SELECT 1 FROM network_interfaces n WHERE n.item_id = i.id AND n.mac::text ILIKE @pattern::text))
  AND (NOT @has_cursor::boolean OR (lower(i.name), i.id) > (@cursor_name::text, @cursor_id::uuid))
ORDER BY lower(i.name), i.id
LIMIT @row_limit;

-- name: CountSearchNetworkDevices :one
SELECT count(*)
FROM storage_items i
JOIN network_devices d ON d.item_id = i.id
WHERE i.kind = 'network_device'
  AND (i.deleted_at IS NOT NULL) = @deleted::boolean
  AND (@pattern::text = '' OR i.name ILIKE @pattern::text OR i.description ILIKE @pattern::text
       OR d.hostname ILIKE @pattern::text OR d.serial_number ILIKE @pattern::text
       OR EXISTS (SELECT 1 FROM network_interfaces n WHERE n.item_id = i.id AND n.mac::text ILIKE @pattern::text));
