-- +goose Up
-- Tablas de moderacion automatica (Fase 3, slice 2 — anti-spam /
-- anti-link / banned-words + settings UI):
--
--   banned_words(group_id, word): PK compuesta, FK CASCADE a
--   groups(telegram_id). El handler normaliza word con LOWER() al
--   insertar (case-insensitive: el matcher lowercase el texto).
--
--   link_allowlist(group_id, domain): PK compuesta, FK CASCADE.
--   Case-preserved en el INSERT (los hosts son case-insensitive en
--   la practica; el matcher lowercases en evaluacion). El helper
--   domainMatches acepta dominio exacto y cualquier subdominio
--   (suffix con "."), evitando "notexample.com" matchear "example.com".
--
-- CHECK constraints (length 1-100 para palabras, 1-253 para dominios
-- segun RFC 1035 max hostname length). Indices por group_id para las
-- queries ListWords/ListAllowlist del Service (pre-load por mensaje).

CREATE TABLE banned_words (
    group_id   BIGINT      NOT NULL REFERENCES groups(telegram_id) ON DELETE CASCADE,
    word       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, word),
    CHECK (length(word) BETWEEN 1 AND 100)
);
CREATE INDEX idx_banned_words_group ON banned_words (group_id);

CREATE TABLE link_allowlist (
    group_id   BIGINT      NOT NULL REFERENCES groups(telegram_id) ON DELETE CASCADE,
    domain     TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, domain),
    CHECK (length(domain) BETWEEN 1 AND 253)
);
CREATE INDEX idx_link_allowlist_group ON link_allowlist (group_id);

-- +goose Down
DROP TABLE IF EXISTS link_allowlist;
DROP TABLE IF EXISTS banned_words;
