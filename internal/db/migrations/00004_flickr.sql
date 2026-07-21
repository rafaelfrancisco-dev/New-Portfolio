-- +goose Up
CREATE TABLE flickr_connections (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    user_id TEXT NOT NULL,
    username TEXT NOT NULL,
    token_ciphertext BLOB NOT NULL,
    secret_ciphertext BLOB NOT NULL,
    connected_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE flickr_oauth_requests (
    state_hash TEXT PRIMARY KEY,
    admin_session_hash TEXT NOT NULL,
    request_token TEXT NOT NULL UNIQUE,
    secret_ciphertext BLOB NOT NULL,
    expires_at DATETIME NOT NULL
);

CREATE TABLE flickr_snapshots (
    id TEXT PRIMARY KEY,
    started_at DATETIME NOT NULL,
    completed_at DATETIME,
    status TEXT NOT NULL CHECK (status IN ('building', 'active', 'failed')),
    error TEXT NOT NULL DEFAULT ''
);

CREATE TABLE flickr_albums (
    snapshot_id TEXT NOT NULL REFERENCES flickr_snapshots(id) ON DELETE CASCADE,
    album_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL,
    PRIMARY KEY (snapshot_id, album_id)
);

CREATE TABLE flickr_photos (
    snapshot_id TEXT NOT NULL REFERENCES flickr_snapshots(id) ON DELETE CASCADE,
    photo_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    page_url TEXT NOT NULL,
    image_small_url TEXT NOT NULL,
    image_large_url TEXT NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    taken_at TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    camera TEXT NOT NULL DEFAULT '',
    lens TEXT NOT NULL DEFAULT '',
    exposure TEXT NOT NULL DEFAULT '',
    aperture TEXT NOT NULL DEFAULT '',
    iso TEXT NOT NULL DEFAULT '',
    focal_length TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (snapshot_id, photo_id)
);

CREATE TABLE flickr_album_photos (
    snapshot_id TEXT NOT NULL,
    album_id TEXT NOT NULL,
    photo_id TEXT NOT NULL,
    position INTEGER NOT NULL,
    PRIMARY KEY (snapshot_id, album_id, photo_id),
    FOREIGN KEY (snapshot_id, album_id) REFERENCES flickr_albums(snapshot_id, album_id) ON DELETE CASCADE,
    FOREIGN KEY (snapshot_id, photo_id) REFERENCES flickr_photos(snapshot_id, photo_id) ON DELETE CASCADE
);

CREATE TABLE flickr_sync_status (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    active_snapshot_id TEXT REFERENCES flickr_snapshots(id),
    last_attempt_at DATETIME,
    last_success_at DATETIME,
    last_error TEXT NOT NULL DEFAULT ''
);
INSERT INTO flickr_sync_status (id) VALUES (1);

CREATE INDEX flickr_album_photos_order ON flickr_album_photos(snapshot_id, album_id, position);

-- +goose Down
DROP TABLE flickr_album_photos;
DROP TABLE flickr_photos;
DROP TABLE flickr_albums;
DROP TABLE flickr_sync_status;
DROP TABLE flickr_snapshots;
DROP TABLE flickr_oauth_requests;
DROP TABLE flickr_connections;
