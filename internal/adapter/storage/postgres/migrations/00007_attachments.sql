-- +goose Up
-- Files attached to issues. The content lives in the row (TOASTed out of
-- line), so backups, transactions and deletes cover it like any other data.
CREATE TABLE attachments (
    id          uuid PRIMARY KEY,
    issue_id    uuid NOT NULL REFERENCES issues (id),
    uploader_id uuid NOT NULL REFERENCES users (id),
    file_name   text NOT NULL,
    media_type  text NOT NULL,
    size        bigint NOT NULL CHECK (size > 0),
    content     bytea NOT NULL,
    created_at  timestamptz NOT NULL,
    CHECK (octet_length(content) = size)
);

-- Uploads are often already compressed (images, archives, PDFs): store
-- them out of line without trying to compress them again.
ALTER TABLE attachments ALTER COLUMN content SET STORAGE EXTERNAL;

-- Serves the per-issue, oldest-first attachment list.
CREATE INDEX attachments_issue_created_idx ON attachments (issue_id, created_at, id);

-- +goose Down
DROP TABLE attachments;
