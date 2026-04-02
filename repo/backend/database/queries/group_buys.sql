-- name: ListGroupBuys :many
SELECT gb.id, gb.item_id, gb.location_id, gb.title, gb.description,
       gb.min_quantity, gb.current_quantity, gb.status, gb.cutoff_at,
       gb.price_per_unit, gb.created_at, gb.updated_at, gb.version,
       i.name AS item_name, i.category AS item_category
FROM group_buys gb
JOIN items i ON i.id = gb.item_id
WHERE ($1::text IS NULL OR gb.status = $1)
  AND ($2::uuid IS NULL OR gb.location_id = $2)
ORDER BY gb.cutoff_at ASC
LIMIT $3 OFFSET $4;

-- name: GetGroupBuyByID :one
SELECT gb.id, gb.item_id, gb.location_id, gb.created_by, gb.title, gb.description,
       gb.min_quantity, gb.current_quantity, gb.status, gb.cutoff_at,
       gb.price_per_unit, gb.notes, gb.created_at, gb.updated_at, gb.version
FROM group_buys gb
WHERE gb.id = $1;

-- name: CreateGroupBuy :one
INSERT INTO group_buys (item_id, location_id, created_by, title, description, min_quantity, cutoff_at, price_per_unit)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateGroupBuyStatus :one
UPDATE group_buys
SET status = $2, updated_at = NOW(), version = version + 1
WHERE id = $1
RETURNING id, status, updated_at, version;

-- name: IncrementGroupBuyQuantity :one
UPDATE group_buys
SET current_quantity = current_quantity + $2,
    updated_at = NOW(),
    version = version + 1
WHERE id = $1
RETURNING id, current_quantity, min_quantity, status;

-- name: GetActiveGroupBuysPastCutoff :many
SELECT id, item_id, location_id, title, min_quantity, current_quantity, cutoff_at
FROM group_buys
WHERE status IN ('published', 'active') AND cutoff_at <= NOW();

-- name: JoinGroupBuy :one
INSERT INTO group_buy_participants (group_buy_id, member_id, quantity)
VALUES ($1, $2, $3)
ON CONFLICT (group_buy_id, member_id) DO UPDATE SET quantity = $3, status = 'committed'
RETURNING *;

-- name: LeaveGroupBuy :exec
UPDATE group_buy_participants
SET status = 'cancelled'
WHERE group_buy_id = $1 AND member_id = $2;

-- name: ListGroupBuyParticipants :many
SELECT gbp.id, gbp.member_id, gbp.quantity, gbp.joined_at, gbp.status,
       u.first_name, u.last_name, u.email
FROM group_buy_participants gbp
JOIN members m ON m.id = gbp.member_id
JOIN users u ON u.id = m.user_id
WHERE gbp.group_buy_id = $1
ORDER BY gbp.joined_at;
