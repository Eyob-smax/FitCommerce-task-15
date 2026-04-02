-- name: GetUserByEmail :one
SELECT id, email, password_hash, first_name, last_name, role, is_active, created_at, updated_at, version
FROM users
WHERE email = $1 AND is_active = true;

-- name: GetUserByID :one
SELECT id, email, password_hash, first_name, last_name, role, is_active, created_at, updated_at, version
FROM users
WHERE id = $1;

-- name: ListUsers :many
SELECT id, email, first_name, last_name, role, is_active, created_at, updated_at
FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CreateUser :one
INSERT INTO users (email, password_hash, first_name, last_name, role)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, email, first_name, last_name, role, is_active, created_at, updated_at, version;

-- name: UpdateUser :one
UPDATE users
SET first_name  = COALESCE(NULLIF($2, ''), first_name),
    last_name   = COALESCE(NULLIF($3, ''), last_name),
    is_active   = COALESCE($4, is_active),
    updated_at  = NOW(),
    version     = version + 1
WHERE id = $1
RETURNING id, email, first_name, last_name, role, is_active, created_at, updated_at, version;

-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3);

-- name: GetRefreshToken :one
SELECT id, user_id, token_hash, expires_at, revoked_at
FROM refresh_tokens
WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW();

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1;

-- name: RevokeAllUserTokens :exec
UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL;
