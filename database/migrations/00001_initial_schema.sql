-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    username TEXT NOT NULL,
    password TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users (username);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users (deleted_at);

CREATE TABLE IF NOT EXISTS feeds (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    url TEXT NOT NULL,
    title TEXT,
    description TEXT,
    last_successfully_fetched_at TIMESTAMPTZ,
    last_error TEXT,
    last_error_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_feeds_url ON feeds (url);
CREATE INDEX IF NOT EXISTS idx_feeds_deleted_at ON feeds (deleted_at);

CREATE TABLE IF NOT EXISTS items (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    feed_id BIGINT NOT NULL,
    title TEXT,
    link TEXT,
    description TEXT,
    content TEXT,
    author TEXT,
    published_at TIMESTAMPTZ,
    guid TEXT,
    CONSTRAINT fk_feeds_items FOREIGN KEY (feed_id) REFERENCES feeds (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_items_feed_id ON items (feed_id);
CREATE INDEX IF NOT EXISTS idx_items_guid ON items (guid);
CREATE INDEX IF NOT EXISTS idx_items_deleted_at ON items (deleted_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS feeds;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
