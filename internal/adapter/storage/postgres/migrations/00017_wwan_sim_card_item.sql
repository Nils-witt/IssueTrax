-- +goose Up
-- A WWAN module can name another storage item as the SIM card it uses.
ALTER TABLE module_wwans
    ADD COLUMN sim_card_item_id uuid REFERENCES storage_items (id) ON DELETE SET NULL;

CREATE INDEX module_wwans_sim_card_item_id_idx ON module_wwans (sim_card_item_id);

-- +goose Down
DROP INDEX module_wwans_sim_card_item_id_idx;

ALTER TABLE module_wwans DROP COLUMN sim_card_item_id;
