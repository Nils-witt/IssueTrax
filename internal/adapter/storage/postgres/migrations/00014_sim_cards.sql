-- +goose Up
-- The SIM card module: a single module holding the provider, phone number
-- and ICCID of a SIM card placed in a device.
ALTER TABLE item_type_module_defaults DROP CONSTRAINT item_type_module_defaults_module_check,
    ADD CONSTRAINT item_type_module_defaults_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'network_interface'));

ALTER TABLE storage_item_modules DROP CONSTRAINT storage_item_modules_module_check,
    ADD CONSTRAINT storage_item_modules_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'network_interface'));

DROP INDEX storage_item_modules_single_key;
CREATE UNIQUE INDEX storage_item_modules_single_key ON storage_item_modules (item_id, module)
    WHERE module IN ('hardware', 'network_host', 'purchase', 'sim_card');

-- Phone numbers and ICCIDs are kept without separators. Neither is unique.
CREATE TABLE module_sim_cards (
    module_id    uuid PRIMARY KEY REFERENCES storage_item_modules (id) ON DELETE CASCADE,
    provider     text NOT NULL DEFAULT '',
    phone_number text NOT NULL DEFAULT '' CHECK (phone_number ~ '^(\+?[0-9]{3,15})?$'),
    iccid        text NOT NULL DEFAULT '' CHECK (iccid ~ '^(89[0-9]{16,20})?$')
);

-- +goose Down
DELETE FROM storage_item_modules WHERE module = 'sim_card';
DELETE FROM item_type_module_defaults WHERE module = 'sim_card';

DROP TABLE module_sim_cards;

DROP INDEX storage_item_modules_single_key;
CREATE UNIQUE INDEX storage_item_modules_single_key ON storage_item_modules (item_id, module)
    WHERE module IN ('hardware', 'network_host', 'purchase');

ALTER TABLE storage_item_modules DROP CONSTRAINT storage_item_modules_module_check,
    ADD CONSTRAINT storage_item_modules_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'network_interface'));

ALTER TABLE item_type_module_defaults DROP CONSTRAINT item_type_module_defaults_module_check,
    ADD CONSTRAINT item_type_module_defaults_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'network_interface'));
