-- +goose Up
-- The central list of manufacturers and their hardware models. Hardware
-- modules reference them instead of keeping free text. Both are only soft
-- deleted; deleted entries do not hold their name.
CREATE TABLE manufacturers (
    id         uuid PRIMARY KEY,
    name       text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    deleted_at timestamptz
);

CREATE UNIQUE INDEX manufacturers_name_key ON manufacturers (lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX manufacturers_name_idx ON manufacturers (lower(name), id);

CREATE TABLE hardware_models (
    id              uuid PRIMARY KEY,
    manufacturer_id uuid NOT NULL REFERENCES manufacturers (id),
    name            text NOT NULL,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    deleted_at      timestamptz,
    -- Lets module_hardware check that its model belongs to its manufacturer.
    CONSTRAINT hardware_models_id_manufacturer_key UNIQUE (id, manufacturer_id)
);

CREATE UNIQUE INDEX hardware_models_manufacturer_name_key ON hardware_models (manufacturer_id, lower(name))
    WHERE deleted_at IS NULL;
CREATE INDEX hardware_models_manufacturer_name_idx ON hardware_models (manufacturer_id, lower(name), id);

ALTER TABLE module_hardware
    ADD COLUMN manufacturer_id uuid REFERENCES manufacturers (id),
    ADD COLUMN model_id uuid,
    ADD CONSTRAINT module_hardware_model_fkey FOREIGN KEY (model_id, manufacturer_id)
        REFERENCES hardware_models (id, manufacturer_id),
    ADD CONSTRAINT module_hardware_model_needs_manufacturer CHECK (model_id IS NULL OR manufacturer_id IS NOT NULL);

CREATE INDEX module_hardware_manufacturer_idx ON module_hardware (manufacturer_id);
CREATE INDEX module_hardware_model_idx ON module_hardware (model_id);

-- Move the free-text values into the list, merging spellings that differ
-- only in case. A model without a manufacturer goes to "Unknown".
INSERT INTO manufacturers (id, name, created_at, updated_at)
SELECT gen_random_uuid(), min(name), now(), now()
FROM (SELECT coalesce(nullif(btrim(manufacturer), ''), 'Unknown') AS name
      FROM module_hardware
      WHERE btrim(manufacturer) <> '' OR btrim(model) <> '') AS used
GROUP BY lower(name);

UPDATE module_hardware h SET manufacturer_id = m.id
FROM manufacturers m
WHERE (btrim(h.manufacturer) <> '' OR btrim(h.model) <> '')
  AND lower(m.name) = lower(coalesce(nullif(btrim(h.manufacturer), ''), 'Unknown'));

INSERT INTO hardware_models (id, manufacturer_id, name, created_at, updated_at)
SELECT gen_random_uuid(), manufacturer_id, min(btrim(model)), now(), now()
FROM module_hardware
WHERE btrim(model) <> ''
GROUP BY manufacturer_id, lower(btrim(model));

UPDATE module_hardware h SET model_id = hm.id
FROM hardware_models hm
WHERE btrim(h.model) <> '' AND hm.manufacturer_id = h.manufacturer_id AND lower(hm.name) = lower(btrim(h.model));

ALTER TABLE module_hardware DROP COLUMN manufacturer, DROP COLUMN model;

-- +goose Down
ALTER TABLE module_hardware
    ADD COLUMN manufacturer text NOT NULL DEFAULT '',
    ADD COLUMN model text NOT NULL DEFAULT '';

UPDATE module_hardware h SET manufacturer = m.name FROM manufacturers m WHERE m.id = h.manufacturer_id;
UPDATE module_hardware h SET model = hm.name FROM hardware_models hm WHERE hm.id = h.model_id;

ALTER TABLE module_hardware DROP COLUMN model_id, DROP COLUMN manufacturer_id;

DROP TABLE hardware_models;
DROP TABLE manufacturers;
