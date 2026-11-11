-- migrate: phase=expand
-- Every message step of a series sends through broadcasts: the step owns
-- a broadcast of kind series, never listed with the team's broadcasts,
-- and each send is a recipient row that names the enrollment and the
-- step visit it belongs to. So series email has the same sending
-- address, unsubscribe links, tracking and reply handling as any
-- broadcast. A person can get the same step again on a later pass, so
-- series rows are unique per visit, while one-off rows keep their
-- uniqueness per broadcast. The narrower keys exist before the old ones
-- go, so a write is never left unchecked.

-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE broadcasts
    ADD COLUMN IF NOT EXISTS kind text NOT NULL DEFAULT 'one_off' CHECK (kind IN ('one_off', 'series')),
    ADD COLUMN IF NOT EXISTS series_id uuid,
    ADD COLUMN IF NOT EXISTS series_node_id text;
ALTER TABLE broadcasts ADD CONSTRAINT broadcasts_series_id_fkey
    FOREIGN KEY (series_id) REFERENCES series ON DELETE CASCADE NOT VALID;
ALTER TABLE broadcasts VALIDATE CONSTRAINT broadcasts_series_id_fkey;

ALTER TABLE broadcast_recipients
    ADD COLUMN IF NOT EXISTS series_enrollment_id uuid,
    ADD COLUMN IF NOT EXISTS series_step_id uuid;
ALTER TABLE broadcast_recipients ADD CONSTRAINT broadcast_recipients_series_enrollment_id_fkey
    FOREIGN KEY (series_enrollment_id) REFERENCES series_enrollments ON DELETE SET NULL NOT VALID;
ALTER TABLE broadcast_recipients VALIDATE CONSTRAINT broadcast_recipients_series_enrollment_id_fkey;
ALTER TABLE broadcast_recipients ADD CONSTRAINT broadcast_recipients_series_step_id_fkey
    FOREIGN KEY (series_step_id) REFERENCES series_steps ON DELETE SET NULL NOT VALID;
ALTER TABLE broadcast_recipients VALIDATE CONSTRAINT broadcast_recipients_series_step_id_fkey;

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS broadcasts_series_id_series_node_id_key
    ON broadcasts (series_id, series_node_id) WHERE series_id IS NOT NULL;
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS broadcast_recipients_one_off_contact_key
    ON broadcast_recipients (broadcast_id, contact_id)
    WHERE contact_id IS NOT NULL AND room_id IS NULL AND series_step_id IS NULL;
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS broadcast_recipients_one_off_room_key
    ON broadcast_recipients (broadcast_id, room_id) WHERE room_id IS NOT NULL AND series_step_id IS NULL;
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS broadcast_recipients_one_off_address_key
    ON broadcast_recipients (broadcast_id, lower(address))
    WHERE address <> '' AND status IN ('queued', 'sent', 'delivered') AND series_step_id IS NULL;
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS broadcast_recipients_series_step_contact_key
    ON broadcast_recipients (series_step_id, contact_id)
    WHERE series_step_id IS NOT NULL AND contact_id IS NOT NULL AND room_id IS NULL;
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS broadcast_recipients_series_step_room_key
    ON broadcast_recipients (series_step_id, room_id) WHERE series_step_id IS NOT NULL AND room_id IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS broadcast_recipients_series_enrollment_id_index
    ON broadcast_recipients (workspace_id, series_enrollment_id) WHERE series_enrollment_id IS NOT NULL;

DROP INDEX CONCURRENTLY IF EXISTS broadcast_recipients_broadcast_id_contact_id_key;
DROP INDEX CONCURRENTLY IF EXISTS broadcast_recipients_broadcast_id_room_id_key;
DROP INDEX CONCURRENTLY IF EXISTS broadcast_recipients_broadcast_id_address_key;

-- +goose Down
CREATE UNIQUE INDEX IF NOT EXISTS broadcast_recipients_broadcast_id_contact_id_key
    ON broadcast_recipients (broadcast_id, contact_id) WHERE contact_id IS NOT NULL AND room_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS broadcast_recipients_broadcast_id_room_id_key
    ON broadcast_recipients (broadcast_id, room_id) WHERE room_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS broadcast_recipients_broadcast_id_address_key
    ON broadcast_recipients (broadcast_id, lower(address)) WHERE address <> '' AND status IN ('queued', 'sent', 'delivered');
DROP INDEX IF EXISTS broadcast_recipients_series_enrollment_id_index;
DROP INDEX IF EXISTS broadcast_recipients_series_step_room_key;
DROP INDEX IF EXISTS broadcast_recipients_series_step_contact_key;
DROP INDEX IF EXISTS broadcast_recipients_one_off_address_key;
DROP INDEX IF EXISTS broadcast_recipients_one_off_room_key;
DROP INDEX IF EXISTS broadcast_recipients_one_off_contact_key;
DROP INDEX IF EXISTS broadcasts_series_id_series_node_id_key;
ALTER TABLE broadcast_recipients DROP COLUMN series_step_id, DROP COLUMN series_enrollment_id;
ALTER TABLE broadcasts DROP COLUMN series_node_id, DROP COLUMN series_id, DROP COLUMN kind;
