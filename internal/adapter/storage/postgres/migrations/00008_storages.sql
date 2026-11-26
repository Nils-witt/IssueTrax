-- +goose Up
-- Physical storage locations (buildings, rooms, shelves, boxes) forming a
-- tree. Storages are global and never deleted.
CREATE TABLE storages (
    id         uuid PRIMARY KEY,
    parent_id  uuid REFERENCES storages (id),
    name       text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT storages_not_own_parent CHECK (parent_id <> id)
);

-- Sibling names are unique ignoring case; root storages (NULL parent) count
-- as siblings of each other. Also serves the per-parent, name-ordered list
-- and the child counts.
CREATE UNIQUE INDEX storages_parent_name_key ON storages (parent_id, lower(name)) NULLS NOT DISTINCT;

-- +goose Down
DROP TABLE storages;
