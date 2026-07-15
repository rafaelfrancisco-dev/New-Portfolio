-- name: ListTodos :many
SELECT * FROM todos ORDER BY id DESC;

-- name: CreateTodo :one
INSERT INTO todos (title) VALUES (?) RETURNING *;

-- name: ToggleTodo :one
UPDATE todos SET completed = NOT completed WHERE id = ? RETURNING *;

-- name: DeleteTodo :execrows
DELETE FROM todos WHERE id = ?;

-- name: CountCompletedBefore :one
SELECT COUNT(*) FROM todos WHERE completed = TRUE AND created_at < ?;

-- name: DeleteCompletedBefore :execrows
DELETE FROM todos WHERE completed = TRUE AND created_at < ?;
