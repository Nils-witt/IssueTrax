-- +goose Up
-- A SIM card module can hold the card's IMSI, kept without separators and
-- not unique.
ALTER TABLE module_sim_cards
    ADD COLUMN imsi text NOT NULL DEFAULT '' CHECK (imsi ~ '^([0-9]{6,15})?$');

-- +goose Down
ALTER TABLE module_sim_cards DROP COLUMN imsi;
