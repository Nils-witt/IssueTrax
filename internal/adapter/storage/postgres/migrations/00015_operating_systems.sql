-- +goose Up
-- The operating system module: a single module holding the name and version
-- of the operating system installed on a device.
ALTER TABLE item_type_module_defaults DROP CONSTRAINT item_type_module_defaults_module_check,
    ADD CONSTRAINT item_type_module_defaults_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'operating_system', 'network_interface'));

ALTER TABLE storage_item_modules DROP CONSTRAINT storage_item_modules_module_check,
    ADD CONSTRAINT storage_item_modules_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'operating_system', 'network_interface'));

DROP INDEX storage_item_modules_single_key;
CREATE UNIQUE INDEX storage_item_modules_single_key ON storage_item_modules (item_id, module)
    WHERE module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'operating_system');

CREATE TABLE module_operating_systems (
    module_id uuid PRIMARY KEY REFERENCES storage_item_modules (id) ON DELETE CASCADE,
    name      text NOT NULL DEFAULT '',
    version   text NOT NULL DEFAULT ''
);

-- +goose Down
DELETE FROM storage_item_modules WHERE module = 'operating_system';
DELETE FROM item_type_module_defaults WHERE module = 'operating_system';

DROP TABLE module_operating_systems;

DROP INDEX storage_item_modules_single_key;
CREATE UNIQUE INDEX storage_item_modules_single_key ON storage_item_modules (item_id, module)
    WHERE module IN ('hardware', 'network_host', 'purchase', 'sim_card');

ALTER TABLE storage_item_modules DROP CONSTRAINT storage_item_modules_module_check,
    ADD CONSTRAINT storage_item_modules_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'network_interface'));

ALTER TABLE item_type_module_defaults DROP CONSTRAINT item_type_module_defaults_module_check,
    ADD CONSTRAINT item_type_module_defaults_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'network_interface'));
