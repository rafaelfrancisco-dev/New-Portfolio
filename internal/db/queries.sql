-- name: ListPosts :many
SELECT * FROM posts ORDER BY published_at DESC;

-- name: ListPostsByCategory :many
SELECT * FROM posts WHERE category = ? ORDER BY published_at DESC;

-- name: ListRecentPosts :many
SELECT * FROM posts ORDER BY published_at DESC LIMIT ?;

-- name: GetPostBySlug :one
SELECT * FROM posts WHERE slug = ?;

-- name: GetOlderPost :one
SELECT * FROM posts WHERE published_at < ? ORDER BY published_at DESC LIMIT 1;

-- name: GetNewerPost :one
SELECT * FROM posts WHERE published_at > ? ORDER BY published_at ASC LIMIT 1;

-- name: CountPostsByYear :many
SELECT CAST(strftime('%Y', published_at) AS TEXT) AS year, COUNT(*) AS count
FROM posts GROUP BY year ORDER BY year DESC;
