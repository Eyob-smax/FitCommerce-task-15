package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestSyncChangesEndpoint(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/sync/changes?since=0&entities=items,group_buys", token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := parseBody(w)
	if body["synced_at"] == nil {
		t.Error("should have synced_at")
	}
	if body["data"] == nil {
		t.Error("should have data")
	}
}

func TestSyncPushEndpoint(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	newName := "Synced Name"

	w := doPost(env.Router, "/api/v1/sync/push", map[string]interface{}{
		"mutations": []map[string]interface{}{
			{
				"idempotency_key": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
				"client_id":       "test-client",
				"entity_type":     "items",
				"entity_id":       seedItemID,
				"operation":       "update",
				"payload":         map[string]interface{}{"name": newName},
			},
		},
	}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseBody(w)["data"].([]interface{})[0].(map[string]interface{})
	if result["status"] != "applied" {
		t.Fatalf("expected applied mutation, got %v", result["status"])
	}

	var actualName string
	err := env.DB.QueryRow(context.Background(), `SELECT name FROM items WHERE id = $1`, seedItemID).Scan(&actualName)
	if err != nil {
		t.Fatalf("failed to query updated item: %v", err)
	}
	if actualName != newName {
		t.Fatalf("expected synced item name %q, got %q", newName, actualName)
	}
}

func TestSyncPushIdempotency(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	payload := map[string]interface{}{
		"mutations": []map[string]interface{}{
			{
				"idempotency_key": "8d0f1f14-8a88-4d22-b50a-ecb39adcf5aa",
				"client_id":       "test-client",
				"entity_type":     "items",
				"entity_id":       seedItemID,
				"operation":       "update",
				"payload":         map[string]interface{}{"price": 29.99},
			},
		},
	}

	// First push
	w1 := doPost(env.Router, "/api/v1/sync/push", payload, token)
	if w1.Code != http.StatusOK {
		t.Fatalf("first push: %d", w1.Code)
	}

	// Second push with same key should succeed (idempotent)
	w2 := doPost(env.Router, "/api/v1/sync/push", payload, token)
	if w2.Code != http.StatusOK {
		t.Fatalf("second push: %d", w2.Code)
	}

	body2 := parseBody(w2)
	result2 := body2["data"].([]interface{})[0].(map[string]interface{})
	if result2["status"] != "applied" {
		t.Fatalf("expected idempotent status applied, got %v", result2["status"])
	}
}

func TestSyncChangesReturnsItems(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/sync/changes?since=0&entities=items", token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSyncChangesMemberExcludesDraftItems(t *testing.T) {
	env := setupRouter(t)
	adminToken := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")
	memberToken := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	createW := doPost(env.Router, "/api/v1/items", map[string]interface{}{
		"name":     fmt.Sprintf("Draft Sync Item %d", time.Now().UnixNano()),
		"category": "gear",
		"price":    12.34,
	}, adminToken)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createW.Code, createW.Body.String())
	}
	createdID := parseBody(createW)["data"].(map[string]interface{})["id"].(string)

	w := doGet(env.Router, "/api/v1/sync/changes?since=0&entities=items", memberToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := parseBody(w)
	data := body["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	for _, raw := range items {
		item := raw.(map[string]interface{})
		if item["id"] == createdID {
			t.Fatalf("member sync should not include draft item %s", createdID)
		}
		if status, ok := item["status"].(string); ok && status != "published" {
			t.Fatalf("member sync should only include published items, got %s", status)
		}
	}
}

func TestSyncChangesMemberExcludesDraftGroupBuys(t *testing.T) {
	env := setupRouter(t)
	adminToken := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")
	memberToken := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	cutoff := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	createW := doPost(env.Router, "/api/v1/group-buys", map[string]interface{}{
		"item_id":        seedItemID,
		"location_id":    seedLocationID,
		"title":          fmt.Sprintf("Draft Sync Group Buy %d", time.Now().UnixNano()),
		"min_quantity":   5,
		"cutoff_at":      cutoff,
		"price_per_unit": 10.0,
	}, adminToken)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createW.Code, createW.Body.String())
	}
	createdID := parseBody(createW)["data"].(map[string]interface{})["id"].(string)

	w := doGet(env.Router, "/api/v1/sync/changes?since=0&entities=group_buys", memberToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := parseBody(w)
	data := body["data"].(map[string]interface{})
	gbs := data["group_buys"].([]interface{})
	for _, raw := range gbs {
		gb := raw.(map[string]interface{})
		if gb["id"] == createdID {
			t.Fatalf("member sync should not include draft group buy %s", createdID)
		}
		if status, ok := gb["status"].(string); ok && status == "draft" {
			t.Fatalf("member sync should not include draft group buys")
		}
	}
}

func TestSyncPushRejectsMemberItemMutation(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	w := doPost(env.Router, "/api/v1/sync/push", map[string]interface{}{
		"mutations": []map[string]interface{}{
			{
				"idempotency_key": "7d6cdb5a-d6ec-4307-b43f-9d197f53e9f1",
				"client_id":       "member-client",
				"entity_type":     "items",
				"entity_id":       seedItemID,
				"operation":       "update",
				"payload":         map[string]interface{}{"name": "Should Not Apply"},
			},
		},
	}, token)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	result := parseBody(w)["data"].([]interface{})[0].(map[string]interface{})
	if result["status"] != "rejected" {
		t.Fatalf("expected rejected status, got %v", result["status"])
	}
}
