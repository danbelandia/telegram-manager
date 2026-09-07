-- +goose Up
-- Slice 2: foto por URL y botones inline (publications-slice2).
-- Sin backfill: las filas existentes del slice 1 quedan con
-- photo_url=NULL y buttons=NULL (publicaciones de texto). Ambas
-- columnas NULL-able, sin defaults. La migracion 00004 (tabla + FK
-- + indices) sigue intacta; este 00005 solo ALTERA.
ALTER TABLE publications ADD COLUMN photo_url TEXT;
ALTER TABLE publications ADD COLUMN buttons   JSONB;

-- +goose Down
-- Quitar columnas nuevas; las filas existentes no se pierden.
ALTER TABLE publications DROP COLUMN buttons;
ALTER TABLE publications DROP COLUMN photo_url;
