-- migrate: phase=expand
-- An article or a collection may be kept to one saved audience. A reader
-- sees an article only when they are in its audience and in the audience
-- of every collection above it; dimmi answers a person only from articles
-- they could read. No audience means everyone.

-- +goose Up
ALTER TABLE collections ADD COLUMN audience_id uuid REFERENCES audiences ON DELETE RESTRICT;
ALTER TABLE articles ADD COLUMN audience_id uuid REFERENCES audiences ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE articles DROP COLUMN audience_id;
ALTER TABLE collections DROP COLUMN audience_id;
