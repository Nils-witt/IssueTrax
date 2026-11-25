-- +goose Up
ALTER TABLE tags ADD COLUMN color text;

-- Spread existing tags over the same palette domain.DefaultTagColor uses.
UPDATE tags SET color = (ARRAY[
    '#d73a4a', '#e99695', '#f9a03f', '#fbca04', '#0e8a16', '#5fbf7f',
    '#006b75', '#1d76db', '#5319e7', '#b60205', '#c5def5', '#d876e3'
])[1 + abs(hashtext(name) % 12)];

ALTER TABLE tags
    ALTER COLUMN color SET NOT NULL,
    ADD CONSTRAINT tags_color_check CHECK (color ~ '^#[0-9a-f]{6}$');

-- +goose Down
ALTER TABLE tags DROP COLUMN color;
