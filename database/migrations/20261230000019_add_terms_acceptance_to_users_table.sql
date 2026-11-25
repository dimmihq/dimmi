-- migrate: phase=expand
-- A person who ticks the acceptance box when creating an account agrees
-- to the terms then in force. The account keeps the proof: which version
-- they accepted, when, and the address and browser the request came
-- from. All four stay empty for an account made without the box, such as
-- one on a self-hosted install or one made from the command line.

-- +goose Up
ALTER TABLE users
    ADD COLUMN terms_version text CHECK (char_length(terms_version) BETWEEN 1 AND 40),
    ADD COLUMN terms_accepted_at timestamptz,
    ADD COLUMN terms_accepted_ip inet,
    ADD COLUMN terms_accepted_user_agent text;

-- +goose Down
ALTER TABLE users
    DROP COLUMN terms_accepted_user_agent,
    DROP COLUMN terms_accepted_ip,
    DROP COLUMN terms_accepted_at,
    DROP COLUMN terms_version;
