-- +migrate Up
-- SQL in section 'Up' is executed when this migration is applied

ALTER TABLE referentials ADD COLUMN organisation_id uuid;

-- +migrate Down
-- SQL section 'Down' is executed when this migration is rolled back

ALTER TABLE referentials DROP COLUMN IF EXISTS organisation_id;