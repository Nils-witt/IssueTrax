-- +goose Up
-- Things kept in a storage. All item kinds share this table; kinds with
-- extra fields get their own table keyed by storage_items.id. Deleting an
-- item only sets deleted_at, so it can be restored.
CREATE TABLE storage_items (
    id          uuid PRIMARY KEY,
    storage_id  uuid NOT NULL REFERENCES storages (id),
    kind        text NOT NULL CONSTRAINT storage_items_kind_check CHECK (kind IN ('generic')),
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL,
    updated_at  timestamptz NOT NULL,
    deleted_at  timestamptz
);

-- Serves the per-storage, name-ordered lists and the item counts.
CREATE INDEX storage_items_storage_name_idx ON storage_items (storage_id, lower(name), id);

-- +goose Down
DROP TABLE storage_items;
