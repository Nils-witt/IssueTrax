-- +goose Up
CREATE TABLE comments (
    id         uuid PRIMARY KEY,
    issue_id   uuid NOT NULL REFERENCES issues (id),
    author_id  uuid NOT NULL REFERENCES users (id),
    body       text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

-- Serves the per-issue, oldest-first comment list.
CREATE INDEX comments_issue_created_idx ON comments (issue_id, created_at, id);

-- +goose Down
DROP TABLE comments;
