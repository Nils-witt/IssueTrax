-- +goose Up
-- The Power over Ethernet role of a network interface: none, provider
-- (supplies power), receiver (can be powered over the cable) or required
-- (must be). Wireless interfaces are always none. Existing interfaces
-- become none.
ALTER TABLE module_network_interfaces
    ADD COLUMN poe text NOT NULL DEFAULT 'none'
        CONSTRAINT module_network_interfaces_poe_check CHECK (poe IN ('none', 'provider', 'receiver', 'required')),
    ADD CONSTRAINT module_network_interfaces_poe_wired_check CHECK (poe = 'none' OR type = 'wired');

-- +goose Down
ALTER TABLE module_network_interfaces DROP COLUMN poe;
