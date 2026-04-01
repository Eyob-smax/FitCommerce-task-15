package audit

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Log writes an immutable audit event. Errors are intentionally swallowed
// so audit failures never interrupt the main request flow.
func Log(
	ctx context.Context,
	db *pgxpool.Pool,
	actorID *string,
	action string,
	entityType string,
	entityID string,
	before interface{},
	after interface{},
	ipAddress string,
) {
	var beforeJSON, afterJSON []byte
	if before != nil {
		beforeJSON, _ = json.Marshal(before)
	}
	if after != nil {
		afterJSON, _ = json.Marshal(after)
	}

	_, _ = db.Exec(ctx, `
		INSERT INTO audit_log (actor_id, action, entity_type, entity_id, before_snapshot, after_snapshot, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, actorID, action, entityType, entityID, safeJSON(beforeJSON), safeJSON(afterJSON), ipAddress)
}

func safeJSON(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return b
}
