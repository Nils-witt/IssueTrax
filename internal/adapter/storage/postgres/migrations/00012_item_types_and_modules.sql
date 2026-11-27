-- +goose Up
-- Storage items v2: every item has an item type and carries modules (groups
-- of fields defined in code) that can be added and removed per item. The
-- generic and network device kinds are dropped without migrating their data.
DROP TABLE issue_storage_items;
DROP TABLE network_interfaces;
DROP TABLE network_devices;
DROP TABLE storage_items;

-- Item types form a tree. Deleting a type only sets deleted_at; its
-- children stay live.
CREATE TABLE item_types (
    id         uuid PRIMARY KEY,
    parent_id  uuid REFERENCES item_types (id),
    name       text NOT NULL,
    tag_prefix text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    deleted_at timestamptz,
    CONSTRAINT item_types_not_own_parent CHECK (parent_id <> id)
);

-- Live sibling names are unique ignoring case (roots are siblings of each
-- other); deleted types do not hold their name.
CREATE UNIQUE INDEX item_types_parent_name_key ON item_types (parent_id, lower(name)) NULLS NOT DISTINCT
    WHERE deleted_at IS NULL;
-- Serves the per-parent, name-ordered lists and the child counts.
CREATE INDEX item_types_parent_idx ON item_types (parent_id, lower(name), id);

-- The modules new items of a type start with (the type's own, without the
-- inherited ones).
CREATE TABLE item_type_module_defaults (
    item_type_id  uuid NOT NULL REFERENCES item_types (id) ON DELETE CASCADE,
    module        text NOT NULL CONSTRAINT item_type_module_defaults_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'network_interface')),
    default_count integer NOT NULL CHECK (default_count >= 1),
    PRIMARY KEY (item_type_id, module)
);

-- The number of the last proposed asset tag used per lower-case prefix.
CREATE TABLE asset_tag_counters (
    prefix text PRIMARY KEY,
    value  bigint NOT NULL CHECK (value >= 0)
);

-- Things kept in a storage. Deleting an item only sets deleted_at.
CREATE TABLE storage_items (
    id                     uuid PRIMARY KEY,
    asset_tag              text NOT NULL,
    storage_id             uuid NOT NULL REFERENCES storages (id),
    item_type_id           uuid NOT NULL REFERENCES item_types (id),
    name                   text NOT NULL,
    description            text NOT NULL DEFAULT '',
    usable                 boolean NOT NULL,
    order_date             timestamptz,
    commissioned_at        timestamptz,
    decommissioned_at      timestamptz,
    decommissioning_reason text NOT NULL DEFAULT '',
    responsible_id         uuid REFERENCES users (id),
    created_at             timestamptz NOT NULL,
    updated_at             timestamptz NOT NULL,
    deleted_at             timestamptz
);

-- Asset tags are unique ignoring case, deleted items included.
CREATE UNIQUE INDEX storage_items_asset_tag_key ON storage_items (lower(asset_tag));
-- Serves the per-storage, name-ordered lists and the item counts.
CREATE INDEX storage_items_storage_name_idx ON storage_items (storage_id, lower(name), id);
CREATE INDEX storage_items_type_idx ON storage_items (item_type_id);
CREATE INDEX storage_items_responsible_idx ON storage_items (responsible_id) WHERE responsible_id IS NOT NULL;

-- One row per module instance; the payload lives in the table of its
-- module. IDs stay the same across updates. There is no position: the
-- display order is computed.
CREATE TABLE storage_item_modules (
    id         uuid PRIMARY KEY,
    item_id    uuid NOT NULL REFERENCES storage_items (id) ON DELETE CASCADE,
    module     text NOT NULL CONSTRAINT storage_item_modules_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'network_interface')),
    is_primary boolean NOT NULL DEFAULT false,
    CONSTRAINT storage_item_modules_primary_multi CHECK (NOT is_primary OR module = 'network_interface')
);

CREATE INDEX storage_item_modules_item_idx ON storage_item_modules (item_id);
-- At most one primary instance per item and module.
CREATE UNIQUE INDEX storage_item_modules_primary_key ON storage_item_modules (item_id, module) WHERE is_primary;
-- Single modules occur at most once per item.
CREATE UNIQUE INDEX storage_item_modules_single_key ON storage_item_modules (item_id, module)
    WHERE module IN ('hardware', 'network_host', 'purchase');

CREATE TABLE module_hardware (
    module_id     uuid PRIMARY KEY REFERENCES storage_item_modules (id) ON DELETE CASCADE,
    manufacturer  text NOT NULL DEFAULT '',
    model         text NOT NULL DEFAULT '',
    serial_number text NOT NULL DEFAULT ''
);

CREATE TABLE module_network_hosts (
    module_id uuid PRIMARY KEY REFERENCES storage_item_modules (id) ON DELETE CASCADE,
    role      text NOT NULL CONSTRAINT module_network_hosts_role_check CHECK (role IN ('router', 'switch', 'client')),
    hostname  text NOT NULL DEFAULT ''
);

-- Prices are kept in hundredths of the currency unit; a price always has
-- an ISO 4217 currency.
CREATE TABLE module_purchases (
    module_id      uuid PRIMARY KEY REFERENCES storage_item_modules (id) ON DELETE CASCADE,
    supplier       text NOT NULL DEFAULT '',
    invoice_number text NOT NULL DEFAULT '',
    price_cents    bigint CHECK (price_cents >= 0),
    currency       text NOT NULL DEFAULT '' CHECK (currency = '' OR currency ~ '^[A-Z]{3}$'),
    warranty_end   timestamptz,
    CONSTRAINT module_purchases_price_currency CHECK ((price_cents IS NULL) = (currency = ''))
);

-- The address columns keep the host address and its prefix length; they
-- and the gateway and DNS columns are only set in static mode. MACs are not
-- unique.
CREATE TABLE module_network_interfaces (
    module_id    uuid PRIMARY KEY REFERENCES storage_item_modules (id) ON DELETE CASCADE,
    name         text NOT NULL,
    mac          macaddr NOT NULL,
    type         text NOT NULL CONSTRAINT module_network_interfaces_type_check CHECK (type IN ('wired', 'wireless')),
    ssid         text NOT NULL DEFAULT '',
    native_vlan  integer CHECK (native_vlan BETWEEN 1 AND 4094),
    tagged_vlans integer[] NOT NULL DEFAULT '{}',
    ipv4_mode    text NOT NULL CONSTRAINT module_network_interfaces_ipv4_mode_check CHECK (ipv4_mode IN ('dhcp', 'static', 'off')),
    ipv4_address inet CHECK (family(ipv4_address) = 4),
    ipv4_gateway inet CHECK (family(ipv4_gateway) = 4),
    ipv4_dns     inet[] NOT NULL DEFAULT '{}',
    ipv6_mode    text NOT NULL CONSTRAINT module_network_interfaces_ipv6_mode_check
        CHECK (ipv6_mode IN ('dhcp', 'slaac', 'static', 'off')),
    ipv6_address inet CHECK (family(ipv6_address) = 6),
    ipv6_gateway inet CHECK (family(ipv6_gateway) = 6),
    ipv6_dns     inet[] NOT NULL DEFAULT '{}'
);

CREATE INDEX module_network_interfaces_mac_idx ON module_network_interfaces (mac);

-- Storage items an issue concerns. Items are only soft deleted, so links to
-- deleted items stay.
CREATE TABLE issue_storage_items (
    issue_id        uuid NOT NULL REFERENCES issues (id),
    storage_item_id uuid NOT NULL REFERENCES storage_items (id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, storage_item_id)
);

-- Serves the item filter and the issues-linking-an-item list.
CREATE INDEX issue_storage_items_item_idx ON issue_storage_items (storage_item_id);

-- +goose Down
-- Restores the empty schema of migrations 00009 to 00011.
DROP TABLE issue_storage_items;
DROP TABLE module_network_interfaces;
DROP TABLE module_purchases;
DROP TABLE module_network_hosts;
DROP TABLE module_hardware;
DROP TABLE storage_item_modules;
DROP TABLE storage_items;
DROP TABLE asset_tag_counters;
DROP TABLE item_type_module_defaults;
DROP TABLE item_types;

CREATE TABLE storage_items (
    id          uuid PRIMARY KEY,
    storage_id  uuid NOT NULL REFERENCES storages (id),
    kind        text NOT NULL CONSTRAINT storage_items_kind_check CHECK (kind IN ('generic', 'network_device')),
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL,
    updated_at  timestamptz NOT NULL,
    deleted_at  timestamptz
);

CREATE INDEX storage_items_storage_name_idx ON storage_items (storage_id, lower(name), id);

CREATE TABLE network_devices (
    item_id       uuid PRIMARY KEY REFERENCES storage_items (id) ON DELETE CASCADE,
    role          text NOT NULL CONSTRAINT network_devices_role_check CHECK (role IN ('router', 'switch', 'client')),
    hostname      text NOT NULL DEFAULT '',
    manufacturer  text NOT NULL DEFAULT '',
    model         text NOT NULL DEFAULT '',
    serial_number text NOT NULL DEFAULT ''
);

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

CREATE TABLE issue_storage_items (
    issue_id        uuid NOT NULL REFERENCES issues (id),
    storage_item_id uuid NOT NULL REFERENCES storage_items (id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, storage_item_id)
);

CREATE INDEX issue_storage_items_item_idx ON issue_storage_items (storage_item_id);
