-- +goose Up
-- The Bluetooth module: a single module describing a device's Bluetooth
-- radio by advertised name, device address (BD_ADDR) and Core Specification
-- version.
ALTER TABLE item_type_module_defaults DROP CONSTRAINT item_type_module_defaults_module_check,
    ADD CONSTRAINT item_type_module_defaults_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system', 'sip_client', 'bluetooth', 'network_interface'));

ALTER TABLE storage_item_modules DROP CONSTRAINT storage_item_modules_module_check,
    ADD CONSTRAINT storage_item_modules_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system', 'sip_client', 'bluetooth', 'network_interface'));

DROP INDEX storage_item_modules_single_key;
CREATE UNIQUE INDEX storage_item_modules_single_key ON storage_item_modules (item_id, module)
    WHERE module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system', 'sip_client', 'bluetooth');

CREATE TABLE module_bluetooth (
    module_id uuid PRIMARY KEY REFERENCES storage_item_modules (id) ON DELETE CASCADE,
    name      text NOT NULL DEFAULT '',
    -- Empty, or an EUI-48 in lower case and colon-separated (macaddr::text form).
    address   text NOT NULL DEFAULT '' CHECK (address ~ '^([0-9a-f]{2}(:[0-9a-f]{2}){5})?$'),
    version   text NOT NULL DEFAULT '' CHECK (version ~ '^([1-9][0-9]?\.[0-9])?$')
);

-- +goose Down
DELETE FROM storage_item_modules WHERE module = 'bluetooth';
DELETE FROM item_type_module_defaults WHERE module = 'bluetooth';

DROP TABLE module_bluetooth;

DROP INDEX storage_item_modules_single_key;
CREATE UNIQUE INDEX storage_item_modules_single_key ON storage_item_modules (item_id, module)
    WHERE module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system', 'sip_client');

ALTER TABLE storage_item_modules DROP CONSTRAINT storage_item_modules_module_check,
    ADD CONSTRAINT storage_item_modules_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system', 'sip_client', 'network_interface'));

ALTER TABLE item_type_module_defaults DROP CONSTRAINT item_type_module_defaults_module_check,
    ADD CONSTRAINT item_type_module_defaults_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system', 'sip_client', 'network_interface'));
