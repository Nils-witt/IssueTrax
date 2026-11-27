-- +goose Up
-- The WWAN module: a single module describing a device's mobile (cellular)
-- connection by modem IMEI, APN and network technology.
ALTER TABLE item_type_module_defaults DROP CONSTRAINT item_type_module_defaults_module_check,
    ADD CONSTRAINT item_type_module_defaults_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system', 'network_interface'));

ALTER TABLE storage_item_modules DROP CONSTRAINT storage_item_modules_module_check,
    ADD CONSTRAINT storage_item_modules_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system', 'network_interface'));

DROP INDEX storage_item_modules_single_key;
CREATE UNIQUE INDEX storage_item_modules_single_key ON storage_item_modules (item_id, module)
    WHERE module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system');

CREATE TABLE module_wwans (
    module_id  uuid PRIMARY KEY REFERENCES storage_item_modules (id) ON DELETE CASCADE,
    imei       text NOT NULL DEFAULT '',
    apn        text NOT NULL DEFAULT '',
    technology text NOT NULL DEFAULT '' CHECK (technology IN ('', '2g', '3g', '4g', '5g'))
);

-- +goose Down
DELETE FROM storage_item_modules WHERE module = 'wwan';
DELETE FROM item_type_module_defaults WHERE module = 'wwan';

DROP TABLE module_wwans;

DROP INDEX storage_item_modules_single_key;
CREATE UNIQUE INDEX storage_item_modules_single_key ON storage_item_modules (item_id, module)
    WHERE module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'operating_system');

ALTER TABLE storage_item_modules DROP CONSTRAINT storage_item_modules_module_check,
    ADD CONSTRAINT storage_item_modules_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'operating_system', 'network_interface'));

ALTER TABLE item_type_module_defaults DROP CONSTRAINT item_type_module_defaults_module_check,
    ADD CONSTRAINT item_type_module_defaults_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'operating_system', 'network_interface'));
