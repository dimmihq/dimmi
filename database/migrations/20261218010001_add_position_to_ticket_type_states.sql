-- migrate: phase=expand
-- Each type keeps its states in its own order, so reordering the steps
-- of one type leaves every other type as it was. Existing rows take the
-- workspace's order of the states, which is what types showed until now.
-- The release before this one inserts rows without a position; they
-- read as first and the next save of the type puts them in place.

-- +goose Up
ALTER TABLE ticket_type_states ADD COLUMN position integer NOT NULL DEFAULT 0;

UPDATE ticket_type_states ts SET position = s.position
FROM ticket_states s
WHERE s.id = ts.state_id;

-- +goose Down
ALTER TABLE ticket_type_states DROP COLUMN position;
