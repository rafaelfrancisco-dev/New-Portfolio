-- +goose Up
CREATE TABLE posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('software', 'photography')),
    teaser TEXT NOT NULL,
    lede TEXT NOT NULL,
    body_md TEXT NOT NULL,
    store_url TEXT NOT NULL DEFAULT '',
    published_at TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE posts;
