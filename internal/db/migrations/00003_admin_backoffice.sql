-- +goose Up
PRAGMA foreign_keys = OFF;

CREATE TABLE posts_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('software', 'photography')),
    teaser TEXT NOT NULL,
    lede TEXT NOT NULL,
    body_md TEXT NOT NULL,
    store_url TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    published_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO posts_new (
    id, slug, title, category, teaser, lede, body_md, store_url,
    status, published_at, created_at, updated_at
)
SELECT
    id, slug, title, category, teaser, lede, body_md, store_url,
    'published', published_at, published_at, CURRENT_TIMESTAMP
FROM posts;

DROP TABLE posts;
ALTER TABLE posts_new RENAME TO posts;

CREATE INDEX posts_publication_idx ON posts(status, published_at DESC);
CREATE INDEX posts_category_idx ON posts(category, status, published_at DESC);

CREATE TABLE admin_sessions (
    token_hash TEXT PRIMARY KEY,
    csrf_token TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL
);
CREATE INDEX admin_sessions_expiry_idx ON admin_sessions(expires_at);

CREATE TABLE login_attempts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ip_address TEXT NOT NULL,
    attempted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX login_attempts_ip_time_idx ON login_attempts(ip_address, attempted_at);

CREATE TABLE slug_redirects (
    old_slug TEXT PRIMARY KEY,
    post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX slug_redirects_post_idx ON slug_redirects(post_id);

CREATE TABLE media (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    filename TEXT NOT NULL UNIQUE,
    original_filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE site_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    display_name TEXT NOT NULL,
    site_handle TEXT NOT NULL,
    site_title TEXT NOT NULL,
    site_description TEXT NOT NULL,
    journal_description TEXT NOT NULL,
    contact_email TEXT NOT NULL,
    github_url TEXT NOT NULL DEFAULT '',
    linkedin_url TEXT NOT NULL DEFAULT '',
    instagram_url TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO site_settings (
    id, display_name, site_handle, site_title, site_description,
    journal_description, contact_email, github_url, linkedin_url, instagram_url
) VALUES (
    1,
    'Rafael Francisco',
    'rafael.francisco',
    'Rafael Francisco — iOS apps & photography',
    'I build iOS apps and photograph the wild bits of Portugal.',
    'Notes from the field & the desk — iOS development and photography in Portugal.',
    'rafael@imaginarymakings.dev',
    'https://github.com/rafaelfrancisco',
    'https://linkedin.com/in/rafaelfrancisco',
    'https://instagram.com/rafaelfrancisco'
);

PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;

DROP TABLE site_settings;
DROP TABLE media;
DROP TABLE slug_redirects;
DROP TABLE login_attempts;
DROP TABLE admin_sessions;

CREATE TABLE posts_old (
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

INSERT INTO posts_old (id, slug, title, category, teaser, lede, body_md, store_url, published_at)
SELECT id, slug, title, category, teaser, lede, body_md, store_url, published_at
FROM posts WHERE published_at IS NOT NULL;

DROP TABLE posts;
ALTER TABLE posts_old RENAME TO posts;

PRAGMA foreign_keys = ON;
