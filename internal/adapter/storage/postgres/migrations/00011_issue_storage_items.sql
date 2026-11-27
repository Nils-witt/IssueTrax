-- +goose Up
-- Storage items (of any kind) an issue concerns. Items are only soft
-- deleted, so links to deleted items stay.
CREATE TABLE issue_storage_items (
    issue_id        uuid NOT NULL REFERENCES issues (id),
    storage_item_id uuid NOT NULL REFERENCES storage_items (id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, storage_item_id)
);

-- Serves the item filter and the issues-linking-an-item list.
CREATE INDEX issue_storage_items_item_idx ON issue_storage_items (storage_item_id);

-- +goose Down
DROP TABLE issue_storage_items;
