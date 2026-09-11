-- +goose Up
ALTER TABLE publications ADD COLUMN video_url TEXT;

-- +goose Down
ALTER TABLE publications DROP COLUMN IF EXISTS video_url;
