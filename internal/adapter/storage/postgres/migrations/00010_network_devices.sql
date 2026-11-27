-- +goose Up
-- Network devices are storage items with a role, optional identification
-- and one or more network interfaces.
ALTER TABLE storage_items
    DROP CONSTRAINT storage_items_kind_check,
    ADD CONSTRAINT storage_items_kind_check CHECK (kind IN ('generic', 'network_device'));

CREATE TABLE network_devices (
    item_id       uuid PRIMARY KEY REFERENCES storage_items (id) ON DELETE CASCADE,
    role          text NOT NULL CONSTRAINT network_devices_role_check CHECK (role IN ('router', 'switch', 'client')),
    hostname      text NOT NULL DEFAULT '',
    manufacturer  text NOT NULL DEFAULT '',
    model         text NOT NULL DEFAULT '',
    serial_number text NOT NULL DEFAULT ''
);

-- The interfaces of a device in order: position 0 is the primary interface,
-- the secondary ones follow. Updates replace all rows of a device. MACs are
-- not unique. The address columns keep the host address and its prefix
-- length; they and the gateway and DNS columns are only set in static mode.
CREATE TABLE network_interfaces (
    item_id      uuid NOT NULL REFERENCES network_devices (item_id) ON DELETE CASCADE,
    position     integer NOT NULL CHECK (position >= 0),
    name         text NOT NULL,
    mac          macaddr NOT NULL,
    type         text NOT NULL CONSTRAINT network_interfaces_type_check CHECK (type IN ('wired', 'wireless')),
    ssid         text NOT NULL DEFAULT '',
    native_vlan  integer CHECK (native_vlan BETWEEN 1 AND 4094),
    tagged_vlans integer[] NOT NULL DEFAULT '{}',
    ipv4_mode    text NOT NULL CONSTRAINT network_interfaces_ipv4_mode_check CHECK (ipv4_mode IN ('dhcp', 'static', 'off')),
    ipv4_address inet CHECK (family(ipv4_address) = 4),
    ipv4_gateway inet CHECK (family(ipv4_gateway) = 4),
    ipv4_dns     inet[] NOT NULL DEFAULT '{}',
    ipv6_mode    text NOT NULL CONSTRAINT network_interfaces_ipv6_mode_check CHECK (ipv6_mode IN ('dhcp', 'slaac', 'static', 'off')),
    ipv6_address inet CHECK (family(ipv6_address) = 6),
    ipv6_gateway inet CHECK (family(ipv6_gateway) = 6),
    ipv6_dns     inet[] NOT NULL DEFAULT '{}',
    PRIMARY KEY (item_id, position)
);

-- +goose Down
DROP TABLE network_interfaces;
DROP TABLE network_devices;
DELETE FROM storage_items WHERE kind = 'network_device';
ALTER TABLE storage_items
    DROP CONSTRAINT storage_items_kind_check,
    ADD CONSTRAINT storage_items_kind_check CHECK (kind IN ('generic'));
