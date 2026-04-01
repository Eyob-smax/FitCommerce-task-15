// Package syncsvc handles offline-sync endpoints.
// Named syncsvc to avoid collision with the stdlib sync package.
package syncsvc

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"fitcommerce/backend/internal/auth"
	"fitcommerce/backend/internal/http/response"
	"fitcommerce/backend/internal/middleware"
)

type Handler struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

func NewHandler(db *pgxpool.Pool, rdb *redis.Client) *Handler {
	return &Handler{db: db, rdb: rdb}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/sync")
	g.GET("/changes", h.changes)
	g.POST("/push", h.push)
	g.POST("/resolve", h.resolve)
}

// ── Types ─────────────────────────────────────────────────────────────────────

type pushRequest struct {
	Mutations []mutationInput `json:"mutations" binding:"required,min=1"`
}

type mutationInput struct {
	IdempotencyKey string                 `json:"idempotency_key" binding:"required"`
	ClientID       string                 `json:"client_id" binding:"required"`
	EntityType     string                 `json:"entity_type" binding:"required"`
	EntityID       string                 `json:"entity_id"`
	Operation      string                 `json:"operation" binding:"required"`
	Payload        map[string]interface{} `json:"payload" binding:"required"`
}

type resolveRequest struct {
	MutationID string `json:"mutation_id" binding:"required"`
	Resolution string `json:"resolution" binding:"required"` // "accept_client" | "accept_server" | "discard"
}

// ── Changes endpoint — delta pull ─────────────────────────────────────────────

func (h *Handler) changes(c *gin.Context) {
	sinceStr := c.DefaultQuery("since", "0")
	sinceUnix, _ := strconv.ParseInt(sinceStr, 10, 64)
	since := time.Unix(sinceUnix, 0)
	role := c.GetString(middleware.KeyRole)

	entities := c.DefaultQuery("entities", "items")
	entityList := strings.Split(entities, ",")

	result := map[string]interface{}{}

	for _, entity := range entityList {
		switch strings.TrimSpace(entity) {
		case "items":
			query := `
				SELECT id, name, description, category, brand, condition, billing_model,
					deposit_amount, price, status, location_id, version, updated_at
				FROM items WHERE updated_at > $1`
			if role == auth.RoleMember || role == auth.RoleCoach {
				query += ` AND status = 'published'`
			}
			query += `
				ORDER BY updated_at LIMIT 500`

			rows, err := h.db.Query(c.Request.Context(), query, since)
			if err == nil {
				result["items"] = scanItems(rows)
			}
		case "group_buys":
			query := `
				SELECT g.id, g.item_id, i.name, g.title, g.description, g.min_quantity,
					g.current_quantity, g.status, g.cutoff_at, g.price_per_unit,
					g.location_id, g.version, g.updated_at
				FROM group_buys g JOIN items i ON i.id = g.item_id
				WHERE g.updated_at > $1`
			if role == auth.RoleMember || role == auth.RoleCoach {
				query += ` AND g.status IN ('published','active','succeeded','failed','fulfilled')`
			}
			query += `
				ORDER BY g.updated_at LIMIT 500`

			rows, err := h.db.Query(c.Request.Context(), query, since)
			if err == nil {
				result["group_buys"] = scanGroupBuys(rows)
			}
		case "orders":
			userID := c.GetString(middleware.KeyUserID)
			rows, err := h.db.Query(c.Request.Context(), `
				SELECT o.id, o.member_id, o.status, o.total_amount, o.deposit_amount,
					o.group_buy_id, o.version, o.updated_at
				FROM orders o
				JOIN members m ON m.id = o.member_id
				WHERE o.updated_at > $1 AND m.user_id = $2
				ORDER BY o.updated_at LIMIT 500
			`, since, userID)
			if err == nil {
				result["orders"] = scanOrders(rows)
			}
		case "members":
			role := c.GetString(middleware.KeyRole)
			userID := c.GetString(middleware.KeyUserID)
			var memberRows pgx.Rows
			var memberErr error
			if role == auth.RoleAdministrator || role == auth.RoleOperationsManager {
				// Staff may sync all member records
				memberRows, memberErr = h.db.Query(c.Request.Context(), `
					SELECT id, user_id, location_id, membership_type, status, version, updated_at
					FROM members WHERE updated_at > $1
					ORDER BY updated_at LIMIT 500
				`, since)
			} else {
				// Non-staff may only sync their own member record
				memberRows, memberErr = h.db.Query(c.Request.Context(), `
					SELECT id, user_id, location_id, membership_type, status, version, updated_at
					FROM members WHERE user_id = $1 AND updated_at > $2
					ORDER BY updated_at LIMIT 500
				`, userID, since)
			}
			if memberErr == nil {
				result["members"] = scanMembers(memberRows)
			}
		}
	}

	now := time.Now().Unix()
	c.JSON(200, gin.H{"data": result, "synced_at": now})
}

// ── Push endpoint — receive mutations ─────────────────────────────────────────

func (h *Handler) push(c *gin.Context) {
	var req pushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	ctx := c.Request.Context()
	role := c.GetString(middleware.KeyRole)
	actorUserID := c.GetString(middleware.KeyUserID)
	results := []map[string]interface{}{}

	for _, m := range req.Mutations {
		// Check idempotency — if already processed, return success
		var existingStatus string
		var existingConflict map[string]interface{}
		err := h.db.QueryRow(ctx, `SELECT status FROM sync_mutations WHERE idempotency_key = $1`,
			m.IdempotencyKey).Scan(&existingStatus)
		if err == nil {
			result := map[string]interface{}{
				"idempotency_key": m.IdempotencyKey,
				"status":          existingStatus,
			}
			if existingStatus == "conflict" && existingConflict != nil {
				result["conflict_data"] = existingConflict
			}
			results = append(results, result)
			continue
		}

		// Insert mutation record
		_, err = h.db.Exec(ctx, `
			INSERT INTO sync_mutations (idempotency_key, client_id, entity_type, entity_id, operation, payload)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, m.IdempotencyKey, m.ClientID, m.EntityType, m.EntityID, m.Operation, m.Payload)
		if err != nil {
			results = append(results, map[string]interface{}{
				"idempotency_key": m.IdempotencyKey,
				"status":          "rejected",
				"error":           "failed to record mutation",
			})
			continue
		}

		status, errMsg, conflictData := h.applyMutation(ctx, m, role, actorUserID)

		if _, err := h.db.Exec(ctx, `
			UPDATE sync_mutations
			SET status = $1, conflict_data = $2, processed_at = NOW()
			WHERE idempotency_key = $3
		`, status, conflictData, m.IdempotencyKey); err != nil {
			status = "rejected"
			errMsg = "failed to persist mutation status"
		}

		result := map[string]interface{}{
			"idempotency_key": m.IdempotencyKey,
			"status":          status,
		}
		if errMsg != "" {
			result["error"] = errMsg
		}
		if conflictData != nil {
			result["conflict_data"] = conflictData
		}
		results = append(results, result)
	}

	response.OK(c, results)
}

// ── Resolve endpoint — conflict resolution ────────────────────────────────────

func (h *Handler) resolve(c *gin.Context) {
	var req resolveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	ctx := c.Request.Context()
	var exists bool
	_ = h.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sync_mutations WHERE id = $1)`, req.MutationID).Scan(&exists)
	if !exists {
		response.NotFound(c, "mutation")
		return
	}

	newStatus := "applied"
	if req.Resolution == "discard" || req.Resolution == "accept_server" {
		newStatus = "rejected"
	}

	_, _ = h.db.Exec(ctx, `UPDATE sync_mutations SET status = $1, processed_at = NOW() WHERE id = $2`,
		newStatus, req.MutationID)

	response.OK(c, gin.H{"mutation_id": req.MutationID, "status": newStatus})
}

func (h *Handler) applyMutation(
	ctx context.Context,
	m mutationInput,
	role string,
	actorUserID string,
) (status string, errMsg string, conflictData map[string]interface{}) {
	switch m.EntityType {
	case "items":
		return h.applyItemMutation(ctx, m, role, actorUserID)
	case "group_buys":
		return h.applyGroupBuyMutation(ctx, m, role, actorUserID)
	default:
		return "rejected", "unsupported entity_type for offline sync", nil
	}
}

func (h *Handler) applyItemMutation(
	ctx context.Context,
	m mutationInput,
	role string,
	actorUserID string,
) (status string, errMsg string, conflictData map[string]interface{}) {
	if role != auth.RoleAdministrator && role != auth.RoleOperationsManager {
		return "rejected", "role is not allowed to mutate items", nil
	}

	switch m.Operation {
	case "create":
		name, ok := payloadString(m.Payload, "name")
		if !ok || strings.TrimSpace(name) == "" {
			return "rejected", "item create requires payload.name", nil
		}
		category, ok := payloadString(m.Payload, "category")
		if !ok || strings.TrimSpace(category) == "" {
			return "rejected", "item create requires payload.category", nil
		}
		price, ok := payloadFloat(m.Payload, "price")
		if !ok {
			return "rejected", "item create requires payload.price", nil
		}
		if price < 0 {
			return "rejected", "price must be >= 0", nil
		}

		condition := payloadStringDefault(m.Payload, "condition", "new")
		if condition != "new" && condition != "open-box" && condition != "used" {
			return "rejected", "condition must be new, open-box, or used", nil
		}

		billingModel := payloadStringDefault(m.Payload, "billing_model", "one-time")
		if billingModel != "one-time" && billingModel != "monthly-rental" {
			return "rejected", "billing_model must be one-time or monthly-rental", nil
		}

		depositAmount := payloadFloatDefault(m.Payload, "deposit_amount", 50.0)
		if depositAmount < 0 {
			return "rejected", "deposit_amount must be >= 0", nil
		}

		images := payloadStringArray(m.Payload, "images")
		if images == nil {
			images = []string{}
		}

		status := payloadStringDefault(m.Payload, "status", "draft")
		if status != "draft" && status != "published" && status != "unpublished" {
			return "rejected", "invalid item status", nil
		}

		var sku *string
		if v, ok := payloadString(m.Payload, "sku"); ok && strings.TrimSpace(v) != "" {
			sku = &v
		}
		var brand *string
		if v, ok := payloadString(m.Payload, "brand"); ok && strings.TrimSpace(v) != "" {
			brand = &v
		}
		var description *string
		if v, ok := payloadString(m.Payload, "description"); ok && strings.TrimSpace(v) != "" {
			description = &v
		}
		var locationID *string
		if v, ok := payloadString(m.Payload, "location_id"); ok && strings.TrimSpace(v) != "" {
			locationID = &v
		}

		query := `
			INSERT INTO items (name, sku, category, brand, condition, description, images, deposit_amount, billing_model, price, status, location_id, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`
		args := []interface{}{name, sku, category, brand, condition, description, images, depositAmount, billingModel, price, status, locationID, actorUserID}

		if m.EntityID != "" {
			query = `
				INSERT INTO items (id, name, sku, category, brand, condition, description, images, deposit_amount, billing_model, price, status, location_id, created_by)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			`
			args = []interface{}{m.EntityID, name, sku, category, brand, condition, description, images, depositAmount, billingModel, price, status, locationID, actorUserID}
		}

		if _, err := h.db.Exec(ctx, query, args...); err != nil {
			return "rejected", "failed to create item", nil
		}
		return "applied", "", nil

	case "update":
		itemID := m.EntityID
		if itemID == "" {
			if id, ok := payloadString(m.Payload, "id"); ok {
				itemID = id
			}
		}
		if itemID == "" {
			return "rejected", "item update requires entity_id or payload.id", nil
		}

		if payloadVersion, ok := payloadInt(m.Payload, "version"); ok {
			var currentVersion int
			err := h.db.QueryRow(ctx, `SELECT version FROM items WHERE id = $1`, itemID).Scan(&currentVersion)
			if err == pgx.ErrNoRows {
				return "rejected", "item not found", nil
			}
			if err != nil {
				return "rejected", "failed to load current item version", nil
			}
			if currentVersion != payloadVersion {
				return "conflict", "item version conflict", map[string]interface{}{
					"entity_type":    "items",
					"entity_id":      itemID,
					"server_version": currentVersion,
					"client_version": payloadVersion,
				}
			}
		}

		sets := []string{"updated_at = NOW()", "version = version + 1"}
		args := []interface{}{}
		argIdx := 1

		if v, ok := payloadString(m.Payload, "name"); ok {
			sets = append(sets, fmt.Sprintf("name = $%d", argIdx))
			args = append(args, v)
			argIdx++
		}
		if v, ok := payloadString(m.Payload, "sku"); ok {
			if strings.TrimSpace(v) == "" {
				sets = append(sets, fmt.Sprintf("sku = NULL"))
			} else {
				sets = append(sets, fmt.Sprintf("sku = $%d", argIdx))
				args = append(args, v)
				argIdx++
			}
		}
		if v, ok := payloadString(m.Payload, "category"); ok {
			sets = append(sets, fmt.Sprintf("category = $%d", argIdx))
			args = append(args, v)
			argIdx++
		}
		if v, ok := payloadString(m.Payload, "brand"); ok {
			if strings.TrimSpace(v) == "" {
				sets = append(sets, "brand = NULL")
			} else {
				sets = append(sets, fmt.Sprintf("brand = $%d", argIdx))
				args = append(args, v)
				argIdx++
			}
		}
		if v, ok := payloadString(m.Payload, "condition"); ok {
			if v != "new" && v != "open-box" && v != "used" {
				return "rejected", "condition must be new, open-box, or used", nil
			}
			sets = append(sets, fmt.Sprintf("condition = $%d", argIdx))
			args = append(args, v)
			argIdx++
		}
		if v, ok := payloadString(m.Payload, "description"); ok {
			if strings.TrimSpace(v) == "" {
				sets = append(sets, "description = NULL")
			} else {
				sets = append(sets, fmt.Sprintf("description = $%d", argIdx))
				args = append(args, v)
				argIdx++
			}
		}
		if v, ok := payloadStringArray(m.Payload, "images"); ok {
			sets = append(sets, fmt.Sprintf("images = $%d", argIdx))
			args = append(args, v)
			argIdx++
		}
		if v, ok := payloadFloat(m.Payload, "deposit_amount"); ok {
			if v < 0 {
				return "rejected", "deposit_amount must be >= 0", nil
			}
			sets = append(sets, fmt.Sprintf("deposit_amount = $%d", argIdx))
			args = append(args, v)
			argIdx++
		}
		if v, ok := payloadString(m.Payload, "billing_model"); ok {
			if v != "one-time" && v != "monthly-rental" {
				return "rejected", "billing_model must be one-time or monthly-rental", nil
			}
			sets = append(sets, fmt.Sprintf("billing_model = $%d", argIdx))
			args = append(args, v)
			argIdx++
		}
		if v, ok := payloadFloat(m.Payload, "price"); ok {
			if v < 0 {
				return "rejected", "price must be >= 0", nil
			}
			sets = append(sets, fmt.Sprintf("price = $%d", argIdx))
			args = append(args, v)
			argIdx++
		}
		if v, ok := payloadString(m.Payload, "status"); ok {
			if v != "draft" && v != "published" && v != "unpublished" {
				return "rejected", "invalid item status", nil
			}
			sets = append(sets, fmt.Sprintf("status = $%d", argIdx))
			args = append(args, v)
			argIdx++
		}
		if v, ok := payloadString(m.Payload, "location_id"); ok {
			if strings.TrimSpace(v) == "" {
				sets = append(sets, "location_id = NULL")
			} else {
				sets = append(sets, fmt.Sprintf("location_id = $%d", argIdx))
				args = append(args, v)
				argIdx++
			}
		}

		if len(sets) == 2 {
			return "rejected", "item update had no mutable fields", nil
		}

		query := fmt.Sprintf("UPDATE items SET %s WHERE id = $%d", strings.Join(sets, ", "), argIdx)
		args = append(args, itemID)

		tag, err := h.db.Exec(ctx, query, args...)
		if err != nil {
			return "rejected", "failed to update item", nil
		}
		if tag.RowsAffected() == 0 {
			return "rejected", "item not found", nil
		}
		return "applied", "", nil

	case "delete":
		itemID := m.EntityID
		if itemID == "" {
			if id, ok := payloadString(m.Payload, "id"); ok {
				itemID = id
			}
		}
		if itemID == "" {
			return "rejected", "item delete requires entity_id or payload.id", nil
		}
		tag, err := h.db.Exec(ctx, `DELETE FROM items WHERE id = $1`, itemID)
		if err != nil {
			return "rejected", "failed to delete item", nil
		}
		if tag.RowsAffected() == 0 {
			return "rejected", "item not found", nil
		}
		return "applied", "", nil

	default:
		return "rejected", "unsupported item operation", nil
	}
}

func (h *Handler) applyGroupBuyMutation(
	ctx context.Context,
	m mutationInput,
	role string,
	actorUserID string,
) (status string, errMsg string, conflictData map[string]interface{}) {
	switch m.Operation {
	case "create":
		if role != auth.RoleAdministrator && role != auth.RoleOperationsManager && role != auth.RoleMember {
			return "rejected", "role is not allowed to create group buys", nil
		}
		itemID, ok := payloadString(m.Payload, "item_id")
		if !ok || strings.TrimSpace(itemID) == "" {
			return "rejected", "group buy create requires payload.item_id", nil
		}
		locationID, ok := payloadString(m.Payload, "location_id")
		if !ok || strings.TrimSpace(locationID) == "" {
			return "rejected", "group buy create requires payload.location_id", nil
		}
		title, ok := payloadString(m.Payload, "title")
		if !ok || strings.TrimSpace(title) == "" {
			return "rejected", "group buy create requires payload.title", nil
		}
		minQty, ok := payloadInt(m.Payload, "min_quantity")
		if !ok || minQty < 1 {
			return "rejected", "group buy create requires min_quantity >= 1", nil
		}
		pricePerUnit, ok := payloadFloat(m.Payload, "price_per_unit")
		if !ok || pricePerUnit < 0 {
			return "rejected", "group buy create requires price_per_unit >= 0", nil
		}
		cutoffAtRaw, ok := payloadString(m.Payload, "cutoff_at")
		if !ok {
			return "rejected", "group buy create requires payload.cutoff_at", nil
		}
		cutoffAt, err := time.Parse(time.RFC3339, cutoffAtRaw)
		if err != nil {
			return "rejected", "cutoff_at must be RFC3339 format", nil
		}

		var desc *string
		if v, ok := payloadString(m.Payload, "description"); ok && strings.TrimSpace(v) != "" {
			desc = &v
		}
		var notes *string
		if v, ok := payloadString(m.Payload, "notes"); ok && strings.TrimSpace(v) != "" {
			notes = &v
		}

		initStatus := "draft"
		if role == auth.RoleMember {
			initStatus = "published"
		}

		query := `
			INSERT INTO group_buys (item_id, location_id, created_by, title, description, min_quantity, status, cutoff_at, price_per_unit, notes)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`
		args := []interface{}{itemID, locationID, actorUserID, title, desc, minQty, initStatus, cutoffAt, pricePerUnit, notes}
		if m.EntityID != "" {
			query = `
				INSERT INTO group_buys (id, item_id, location_id, created_by, title, description, min_quantity, status, cutoff_at, price_per_unit, notes)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			`
			args = []interface{}{m.EntityID, itemID, locationID, actorUserID, title, desc, minQty, initStatus, cutoffAt, pricePerUnit, notes}
		}
		if _, err := h.db.Exec(ctx, query, args...); err != nil {
			return "rejected", "failed to create group buy", nil
		}
		return "applied", "", nil

	case "update":
		action, ok := payloadString(m.Payload, "action")
		if !ok {
			return "rejected", "group buy update requires payload.action", nil
		}
		gbID := m.EntityID
		if gbID == "" {
			if id, ok := payloadString(m.Payload, "group_buy_id"); ok {
				gbID = id
			}
		}
		if gbID == "" {
			return "rejected", "group buy mutation requires entity_id or payload.group_buy_id", nil
		}

		switch action {
		case "join":
			if role != auth.RoleMember {
				return "rejected", "only members can join group buys", nil
			}
			qty := 1
			if v, ok := payloadInt(m.Payload, "quantity"); ok && v > 0 {
				qty = v
			}
			return h.applyGroupBuyJoin(ctx, gbID, actorUserID, qty)
		case "leave":
			if role != auth.RoleMember {
				return "rejected", "only members can leave group buys", nil
			}
			return h.applyGroupBuyLeave(ctx, gbID, actorUserID)
		default:
			return "rejected", "unsupported group buy action", nil
		}

	default:
		return "rejected", "unsupported group buy operation", nil
	}
}

func (h *Handler) applyGroupBuyJoin(ctx context.Context, gbID, actorUserID string, qty int) (string, string, map[string]interface{}) {
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return "rejected", "failed to start transaction", nil
	}
	defer tx.Rollback(ctx)

	var status string
	var cutoffAt time.Time
	err = tx.QueryRow(ctx, `SELECT status, cutoff_at FROM group_buys WHERE id = $1 FOR UPDATE`, gbID).Scan(&status, &cutoffAt)
	if err == pgx.ErrNoRows {
		return "rejected", "group buy not found", nil
	}
	if err != nil {
		return "rejected", "failed to load group buy", nil
	}
	if status != "published" && status != "active" {
		return "rejected", "group buy is not open for joining", nil
	}
	if time.Now().After(cutoffAt) {
		return "rejected", "group buy cutoff has passed", nil
	}

	var memberID string
	err = tx.QueryRow(ctx, `SELECT id FROM members WHERE user_id = $1 AND status = 'active'`, actorUserID).Scan(&memberID)
	if err == pgx.ErrNoRows {
		return "rejected", "active member record not found", nil
	}
	if err != nil {
		return "rejected", "failed to load member", nil
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO group_buy_participants (group_buy_id, member_id, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (group_buy_id, member_id) DO UPDATE SET status = 'committed', quantity = $3
	`, gbID, memberID, qty); err != nil {
		return "rejected", "failed to upsert participant", nil
	}

	if _, err := tx.Exec(ctx, `
		UPDATE group_buys SET
			current_quantity = (SELECT COALESCE(SUM(quantity), 0) FROM group_buy_participants WHERE group_buy_id = $1 AND status = 'committed'),
			status = CASE WHEN status = 'published' THEN 'active' ELSE status END,
			updated_at = NOW(),
			version = version + 1
		WHERE id = $1
	`, gbID); err != nil {
		return "rejected", "failed to update group buy totals", nil
	}

	if err := tx.Commit(ctx); err != nil {
		return "rejected", "failed to commit mutation", nil
	}
	return "applied", "", nil
}

func (h *Handler) applyGroupBuyLeave(ctx context.Context, gbID, actorUserID string) (string, string, map[string]interface{}) {
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return "rejected", "failed to start transaction", nil
	}
	defer tx.Rollback(ctx)

	var memberID string
	err = tx.QueryRow(ctx, `SELECT id FROM members WHERE user_id = $1`, actorUserID).Scan(&memberID)
	if err == pgx.ErrNoRows {
		return "rejected", "member record not found", nil
	}
	if err != nil {
		return "rejected", "failed to load member", nil
	}

	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM group_buys WHERE id = $1 FOR UPDATE`, gbID).Scan(&status)
	if err == pgx.ErrNoRows {
		return "rejected", "group buy not found", nil
	}
	if err != nil {
		return "rejected", "failed to load group buy", nil
	}
	if status != "published" && status != "active" {
		return "rejected", "cannot leave a group buy in terminal state", nil
	}

	tag, err := tx.Exec(ctx, `
		UPDATE group_buy_participants SET status = 'cancelled'
		WHERE group_buy_id = $1 AND member_id = $2 AND status = 'committed'
	`, gbID, memberID)
	if err != nil {
		return "rejected", "failed to update participant", nil
	}
	if tag.RowsAffected() == 0 {
		return "rejected", "participation not found", nil
	}

	if _, err := tx.Exec(ctx, `
		UPDATE group_buys SET
			current_quantity = (SELECT COALESCE(SUM(quantity), 0) FROM group_buy_participants WHERE group_buy_id = $1 AND status = 'committed'),
			updated_at = NOW(),
			version = version + 1
		WHERE id = $1
	`, gbID); err != nil {
		return "rejected", "failed to update group buy totals", nil
	}

	if err := tx.Commit(ctx); err != nil {
		return "rejected", "failed to commit mutation", nil
	}
	return "applied", "", nil
}

func payloadString(payload map[string]interface{}, key string) (string, bool) {
	v, ok := payload[key]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}

func payloadStringDefault(payload map[string]interface{}, key, fallback string) string {
	if v, ok := payloadString(payload, key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func payloadFloat(payload map[string]interface{}, key string) (float64, bool) {
	v, ok := payload[key]
	if !ok || v == nil {
		return 0, false
	}
	switch num := v.(type) {
	case float64:
		return num, true
	case float32:
		return float64(num), true
	case int:
		return float64(num), true
	case int32:
		return float64(num), true
	case int64:
		return float64(num), true
	case string:
		parsed, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func payloadFloatDefault(payload map[string]interface{}, key string, fallback float64) float64 {
	if v, ok := payloadFloat(payload, key); ok {
		return v
	}
	return fallback
}

func payloadInt(payload map[string]interface{}, key string) (int, bool) {
	v, ok := payloadFloat(payload, key)
	if !ok {
		return 0, false
	}
	return int(v), true
}

func payloadStringArray(payload map[string]interface{}, key string) ([]string, bool) {
	v, ok := payload[key]
	if !ok || v == nil {
		return nil, false
	}

	if typed, ok := v.([]string); ok {
		return typed, true
	}

	raw, ok := v.([]interface{})
	if !ok {
		return nil, false
	}

	result := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		result = append(result, s)
	}
	return result, true
}

// ── Scan helpers ──────────────────────────────────────────────────────────────

func scanItems(rows pgx.Rows) []map[string]interface{} {
	defer rows.Close()
	var results []map[string]interface{}
	for rows.Next() {
		var id, name, category, condition, billingModel, status string
		var description, brand, locationID *string
		var depositAmount, price float64
		var version int
		var updatedAt time.Time
		if rows.Scan(&id, &name, &description, &category, &brand, &condition,
			&billingModel, &depositAmount, &price, &status, &locationID, &version, &updatedAt) == nil {
			results = append(results, map[string]interface{}{
				"id": id, "name": name, "description": description, "category": category,
				"brand": brand, "condition": condition, "billing_model": billingModel,
				"deposit_amount": depositAmount, "price": price, "status": status,
				"location_id": locationID, "version": version,
				"updated_at": updatedAt.UTC().Format(time.RFC3339),
			})
		}
	}
	return results
}

func scanGroupBuys(rows pgx.Rows) []map[string]interface{} {
	defer rows.Close()
	var results []map[string]interface{}
	for rows.Next() {
		var id, itemID, itemName, title, status, locationID string
		var description *string
		var minQty, curQty, version int
		var cutoffAt, updatedAt time.Time
		var pricePerUnit float64
		if rows.Scan(&id, &itemID, &itemName, &title, &description, &minQty,
			&curQty, &status, &cutoffAt, &pricePerUnit, &locationID, &version, &updatedAt) == nil {
			results = append(results, map[string]interface{}{
				"id": id, "item_id": itemID, "item_name": itemName, "title": title,
				"description": description, "min_quantity": minQty, "current_quantity": curQty,
				"status": status, "cutoff_at": cutoffAt.UTC().Format(time.RFC3339),
				"price_per_unit": pricePerUnit, "location_id": locationID,
				"version": version, "updated_at": updatedAt.UTC().Format(time.RFC3339),
			})
		}
	}
	return results
}

func scanOrders(rows pgx.Rows) []map[string]interface{} {
	defer rows.Close()
	var results []map[string]interface{}
	for rows.Next() {
		var id, memberID, status string
		var totalAmount, depositAmount float64
		var groupBuyID *string
		var version int
		var updatedAt time.Time
		if rows.Scan(&id, &memberID, &status, &totalAmount, &depositAmount,
			&groupBuyID, &version, &updatedAt) == nil {
			results = append(results, map[string]interface{}{
				"id": id, "member_id": memberID, "status": status,
				"total_amount": totalAmount, "deposit_amount": depositAmount,
				"group_buy_id": groupBuyID, "version": version,
				"updated_at": updatedAt.UTC().Format(time.RFC3339),
			})
		}
	}
	return results
}

func scanMembers(rows pgx.Rows) []map[string]interface{} {
	defer rows.Close()
	var results []map[string]interface{}
	for rows.Next() {
		var id, userID, membershipType, status string
		var locationID *string
		var version int
		var updatedAt time.Time
		if rows.Scan(&id, &userID, &locationID, &membershipType, &status, &version, &updatedAt) == nil {
			results = append(results, map[string]interface{}{
				"id": id, "user_id": userID, "location_id": locationID,
				"membership_type": membershipType, "status": status,
				"version": version, "updated_at": updatedAt.UTC().Format(time.RFC3339),
			})
		}
	}
	return results
}
