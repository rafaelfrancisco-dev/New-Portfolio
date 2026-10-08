-- +goose Up
ALTER TABLE admin_sessions ADD COLUMN credential_fingerprint TEXT NOT NULL DEFAULT '';
ALTER TABLE admin_sessions ADD COLUMN generation INTEGER NOT NULL DEFAULT -1;
CREATE TABLE admin_security (id INTEGER PRIMARY KEY CHECK(id=1), generation INTEGER NOT NULL);
INSERT INTO admin_security VALUES(1,0);
CREATE INDEX login_attempts_time_idx ON login_attempts(attempted_at);
CREATE INDEX flickr_photos_order_idx ON flickr_photos(snapshot_id,taken_at DESC,photo_id DESC);
CREATE INDEX flickr_membership_idx ON flickr_album_photos(snapshot_id,photo_id,album_id);
ALTER TABLE flickr_snapshots ADD COLUMN featured_photo_id TEXT;
UPDATE flickr_snapshots SET featured_photo_id=(SELECT photo_id FROM flickr_photos WHERE snapshot_id=flickr_snapshots.id ORDER BY taken_at DESC,photo_id DESC LIMIT 1);
-- +goose Down
ALTER TABLE flickr_snapshots DROP COLUMN featured_photo_id;
DROP INDEX flickr_membership_idx;
DROP INDEX flickr_photos_order_idx;
DROP INDEX login_attempts_time_idx;
DROP TABLE admin_security;
ALTER TABLE admin_sessions DROP COLUMN generation;
ALTER TABLE admin_sessions DROP COLUMN credential_fingerprint;
