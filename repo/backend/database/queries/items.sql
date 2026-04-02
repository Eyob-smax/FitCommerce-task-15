-- name: ListItems :many
SELECT id, name, description, category, brand, condition, billing_model,
       deposit_amount, price, status, location_id, created_by, created_at, updated_at, version
FROM items
WHERE ($1::text IS NULL OR status = $1)
  AND ($2::uuid IS NULL OR location_id = $2)
  AND ($3::text IS NULL OR category = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: GetItemByID :one
SELECT id, name, description, category, brand, condition, billing_model,
       deposit_amount, price, status, location_id, created_by, created_at, updated_at, version
FROM items
WHERE id = $1;

-- name: CreateItem :one
INSERT INTO items (name, description, category, brand, condition, billing_model, deposit_amount, price, status, location_id, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: UpdateItemStatus :one
UPDATE items
SET status = $2, updated_at = NOW(), version = version + 1
WHERE id = $1
RETURNING id, name, status, updated_at, version;

-- name: GetItemAvailabilityWindows :many
SELECT id, item_id, starts_at, ends_at, created_at
FROM item_availability_windows
WHERE item_id = $1
ORDER BY starts_at;

-- name: CreateAvailabilityWindow :one
INSERT INTO item_availability_windows (item_id, starts_at, ends_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: DeleteAvailabilityWindow :exec
DELETE FROM item_availability_windows WHERE id = $1 AND item_id = $2;
