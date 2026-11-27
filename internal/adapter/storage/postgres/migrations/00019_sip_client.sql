-- +goose Up
-- The SIP client module: a single module describing a device's SIP (VoIP)
-- account by server, port, transport, username and extension. Passwords are
-- not stored.
ALTER TABLE item_type_module_defaults DROP CONSTRAINT item_type_module_defaults_module_check,
    ADD CONSTRAINT item_type_module_defaults_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system', 'sip_client', 'network_interface'));

ALTER TABLE storage_item_modules DROP CONSTRAINT storage_item_modules_module_check,
    ADD CONSTRAINT storage_item_modules_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system', 'sip_client', 'network_interface'));

DROP INDEX storage_item_modules_single_key;
CREATE UNIQUE INDEX storage_item_modules_single_key ON storage_item_modules (item_id, module)
    WHERE module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system', 'sip_client');

CREATE TABLE module_sip_clients (
    module_id uuid PRIMARY KEY REFERENCES storage_item_modules (id) ON DELETE CASCADE,
    server    text    NOT NULL DEFAULT '',
    port      integer NOT NULL DEFAULT 0 CHECK (port BETWEEN 0 AND 65535),
    transport text    NOT NULL DEFAULT '' CHECK (transport IN ('', 'udp', 'tcp', 'tls')),
    username  text    NOT NULL DEFAULT '',
    extension text    NOT NULL DEFAULT '' CHECK (extension ~ '^(\+?[0-9]{1,15})?$')
);

-- +goose Down
DELETE FROM storage_item_modules WHERE module = 'sip_client';
DELETE FROM item_type_module_defaults WHERE module = 'sip_client';

DROP TABLE module_sip_clients;

DROP INDEX storage_item_modules_single_key;
CREATE UNIQUE INDEX storage_item_modules_single_key ON storage_item_modules (item_id, module)
    WHERE module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system');

ALTER TABLE storage_item_modules DROP CONSTRAINT storage_item_modules_module_check,
    ADD CONSTRAINT storage_item_modules_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system', 'network_interface'));

ALTER TABLE item_type_module_defaults DROP CONSTRAINT item_type_module_defaults_module_check,
    ADD CONSTRAINT item_type_module_defaults_module_check
        CHECK (module IN ('hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system', 'network_interface'));
