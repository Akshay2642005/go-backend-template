-- Initial schema for the boilerplate backend.
--
-- Users are authenticated and managed by Clerk; the backend persists the
-- subset of profile data it needs (rendering emails, personalization).
-- PostgreSQL 13+ provides gen_random_uuid() without extra extensions.

CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clerk_id   TEXT NOT NULL UNIQUE,
    email      TEXT NOT NULL UNIQUE,
    first_name TEXT NOT NULL DEFAULT '',
    last_name  TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_clerk_id ON users (clerk_id);
CREATE INDEX idx_users_email ON users (email);

---- create above / drop below ----

DROP TABLE IF EXISTS users;
