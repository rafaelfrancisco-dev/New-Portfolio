-- name: ListPosts :many
SELECT * FROM posts
WHERE status = 'published' AND published_at IS NOT NULL
ORDER BY published_at DESC;

-- name: ListPostsByCategory :many
SELECT * FROM posts
WHERE status = 'published' AND published_at IS NOT NULL AND category = ?
ORDER BY published_at DESC;

-- name: ListRecentPosts :many
SELECT * FROM posts
WHERE status = 'published' AND published_at IS NOT NULL
ORDER BY published_at DESC LIMIT ?;

-- name: GetPostBySlug :one
SELECT * FROM posts
WHERE slug = ? AND status = 'published' AND published_at IS NOT NULL;

-- name: GetOlderPost :one
SELECT * FROM posts
WHERE status = 'published' AND published_at IS NOT NULL AND published_at < ?
ORDER BY published_at DESC LIMIT 1;

-- name: GetNewerPost :one
SELECT * FROM posts
WHERE status = 'published' AND published_at IS NOT NULL AND published_at > ?
ORDER BY published_at ASC LIMIT 1;

-- name: CountPostsByYear :many
SELECT CAST(strftime('%Y', published_at) AS TEXT) AS year, COUNT(*) AS count
FROM posts
WHERE status = 'published' AND published_at IS NOT NULL
GROUP BY year ORDER BY year DESC;

-- name: ListAdminPosts :many
SELECT * FROM posts
WHERE (sqlc.arg(status_filter) = '' OR status = sqlc.arg(status_filter))
  AND (sqlc.arg(category_filter) = '' OR category = sqlc.arg(category_filter))
  AND (
    sqlc.arg(search_text) = ''
    OR title LIKE '%' || sqlc.arg(search_text) || '%'
    OR teaser LIKE '%' || sqlc.arg(search_text) || '%'
    OR body_md LIKE '%' || sqlc.arg(search_text) || '%'
  )
ORDER BY updated_at DESC, id DESC;

-- name: GetAdminPost :one
SELECT * FROM posts WHERE id = ?;

-- name: CreatePost :one
INSERT INTO posts (
    slug, title, category, teaser, lede, body_md, store_url,
    status, published_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING *;

-- name: UpdatePost :one
UPDATE posts SET
    slug = ?,
    title = ?,
    category = ?,
    teaser = ?,
    lede = ?,
    body_md = ?,
    store_url = ?,
    status = ?,
    published_at = ?,
    updated_at = strftime('%Y-%m-%d %H:%M:%f', 'now')
WHERE id = ? AND julianday(updated_at) = julianday(sqlc.arg(updated_at))
RETURNING *;

-- name: DeletePost :execrows
DELETE FROM posts WHERE id = ? AND julianday(updated_at) = julianday(sqlc.arg(updated_at));

-- name: SlugInUse :one
SELECT EXISTS(
    SELECT 1 FROM posts WHERE slug = ? AND id != ?
    UNION ALL
    SELECT 1 FROM slug_redirects WHERE old_slug = ? AND post_id != ?
) AS in_use;

-- name: CreateSlugRedirect :exec
INSERT INTO slug_redirects (old_slug, post_id)
VALUES (?, ?)
ON CONFLICT(old_slug) DO UPDATE SET post_id = excluded.post_id;

-- name: DeleteSlugRedirectForCurrentSlug :exec
DELETE FROM slug_redirects WHERE old_slug = ?;

-- name: GetSlugRedirect :one
SELECT p.* FROM slug_redirects r
JOIN posts p ON p.id = r.post_id
WHERE r.old_slug = ? AND p.status = 'published' AND p.published_at IS NOT NULL;

-- name: CreateAdminSession :exec
INSERT INTO admin_sessions (token_hash, csrf_token, expires_at) VALUES (?, ?, ?);

-- name: GetAdminSession :one
SELECT * FROM admin_sessions WHERE token_hash = ?;

-- name: DeleteAdminSession :exec
DELETE FROM admin_sessions WHERE token_hash = ?;

-- name: RecordLoginAttempt :exec
INSERT INTO login_attempts (ip_address) VALUES (?);

-- name: CountRecentLoginAttempts :one
SELECT COUNT(*) FROM login_attempts
WHERE ip_address = ? AND unixepoch(attempted_at) >= unixepoch(sqlc.arg(cutoff));

-- name: ClearLoginAttempts :exec
DELETE FROM login_attempts WHERE ip_address = ?;

-- name: DeleteOldLoginAttempts :exec
DELETE FROM login_attempts WHERE unixepoch(attempted_at) < unixepoch(sqlc.arg(cutoff));

-- name: GetSiteSettings :one
SELECT * FROM site_settings WHERE id = 1;

-- name: UpdateSiteSettings :one
UPDATE site_settings SET
    display_name = ?,
    site_handle = ?,
    site_title = ?,
    site_description = ?,
    journal_description = ?,
    contact_email = ?,
    github_url = ?,
    linkedin_url = ?,
    instagram_url = ?,
    updated_at = strftime('%Y-%m-%d %H:%M:%f', 'now')
WHERE id = 1
RETURNING *;

-- name: CreateMedia :one
INSERT INTO media (filename, original_filename, mime_type, size_bytes)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: ListMedia :many
SELECT
    m.*,
    EXISTS(SELECT 1 FROM posts p WHERE instr(p.body_md, '/media/' || m.filename) > 0) AS in_use
FROM media m
ORDER BY m.created_at DESC, m.id DESC;

-- name: GetMedia :one
SELECT * FROM media WHERE id = ?;

-- name: GetMediaByFilename :one
SELECT * FROM media WHERE filename = ?;

-- name: MediaIsReferenced :one
SELECT EXISTS(
    SELECT 1 FROM posts WHERE instr(body_md, '/media/' || ?) > 0
) AS in_use;

-- name: DeleteMedia :exec
DELETE FROM media WHERE id = ?;

-- name: CountAdminPosts :one
SELECT
    COUNT(*) AS total,
    SUM(CASE WHEN status = 'published' THEN 1 ELSE 0 END) AS published,
    SUM(CASE WHEN status = 'draft' THEN 1 ELSE 0 END) AS drafts
FROM posts;

-- name: CountMedia :one
SELECT COUNT(*) FROM media;
