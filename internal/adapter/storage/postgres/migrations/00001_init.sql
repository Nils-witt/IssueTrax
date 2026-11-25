-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE users (
    id            uuid PRIMARY KEY,
    issuer        text NOT NULL,
    subject       text NOT NULL,
    email         text NOT NULL,
    display_name  text NOT NULL,
    created_at    timestamptz NOT NULL,
    last_login_at timestamptz NOT NULL,
    CONSTRAINT users_identity_key UNIQUE (issuer, subject)
);

CREATE TABLE projects (
    id                uuid PRIMARY KEY,
    key               text NOT NULL,
    name              text NOT NULL,
    description       text NOT NULL DEFAULT '',
    next_issue_number bigint NOT NULL DEFAULT 1,
    created_at        timestamptz NOT NULL,
    updated_at        timestamptz NOT NULL,
    CONSTRAINT projects_key_key UNIQUE (key)
);

-- Project names are unique ignoring case; also serves the name-ordered list.
CREATE UNIQUE INDEX projects_name_lower_key ON projects (lower(name));

CREATE TYPE issue_status AS ENUM ('draft', 'open', 'closed', 'archived');

CREATE TABLE issues (
    id          uuid PRIMARY KEY,
    project_id  uuid NOT NULL REFERENCES projects (id),
    number      bigint NOT NULL,
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    status      issue_status NOT NULL DEFAULT 'open',
    created_by  uuid NOT NULL REFERENCES users (id),
    created_at  timestamptz NOT NULL,
    updated_at  timestamptz NOT NULL,
    CONSTRAINT issues_project_number_key UNIQUE (project_id, number)
);

CREATE INDEX issues_project_status_number_idx ON issues (project_id, status, number DESC);
-- Substring search (ILIKE '%term%') on title and description.
CREATE INDEX issues_title_trgm_idx ON issues USING gin (title gin_trgm_ops);
CREATE INDEX issues_description_trgm_idx ON issues USING gin (description gin_trgm_ops);

CREATE TABLE tags (
    id         uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects (id),
    name       text NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT tags_project_name_key UNIQUE (project_id, name)
);

CREATE TABLE issue_tags (
    issue_id uuid NOT NULL REFERENCES issues (id),
    tag_id   uuid NOT NULL REFERENCES tags (id),
    PRIMARY KEY (issue_id, tag_id)
);

CREATE INDEX issue_tags_tag_idx ON issue_tags (tag_id);

CREATE TABLE issue_assignees (
    issue_id uuid NOT NULL REFERENCES issues (id),
    user_id  uuid NOT NULL REFERENCES users (id),
    PRIMARY KEY (issue_id, user_id)
);

CREATE INDEX issue_assignees_user_idx ON issue_assignees (user_id);

-- +goose Down
DROP TABLE issue_assignees;
DROP TABLE issue_tags;
DROP TABLE tags;
DROP TABLE issues;
DROP TYPE issue_status;
DROP TABLE projects;
DROP TABLE users;
