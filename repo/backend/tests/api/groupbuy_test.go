package api_test

import (
	"net/http"
	"testing"
	"time"
)

// Seeds: active group-buy 88888888-...-001, member 33333333-...-001, item 66666666-...-001

const (
	seedGroupBuyID = "88888888-0000-0000-0000-000000000001"
	seedMemberUID  = "22222222-0000-0000-0000-000000000005" // member user
)

func TestListGroupBuys(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/group-buys", token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetGroupBuy(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/group-buys/"+seedGroupBuyID, token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := parseBody(w)["data"].(map[string]interface{})
	if data["status"] != "active" {
		t.Errorf("expected active, got %v", data["status"])
	}
	if data["progress"] == nil {
		t.Error("expected progress field")
	}
}

func TestCreateGroupBuyAsAdmin(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	cutoff := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	w := doPost(env.Router, "/api/v1/group-buys", map[string]interface{}{
		"item_id":        seedItemID,
		"location_id":    seedLocationID,
		"title":          "Test Group Buy",
		"min_quantity":    5,
		"cutoff_at":      cutoff,
		"price_per_unit": 15.99,
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	data := parseBody(w)["data"].(map[string]interface{})
	if data["status"] != "draft" {
		t.Errorf("admin-created GB should be draft, got %v", data["status"])
	}
}

func TestCreateGroupBuyAsMember(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	cutoff := time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339)
	w := doPost(env.Router, "/api/v1/group-buys", map[string]interface{}{
		"item_id":        seedItemID,
		"location_id":    seedLocationID,
		"title":          "Member Group Buy",
		"min_quantity":    3,
		"cutoff_at":      cutoff,
		"price_per_unit": 10.00,
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	data := parseBody(w)["data"].(map[string]interface{})
	// Members create directly as published
	if data["status"] != "published" {
		t.Errorf("member-created GB should be published, got %v", data["status"])
	}
}

func TestJoinGroupBuy(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	w := doPost(env.Router, "/api/v1/group-buys/"+seedGroupBuyID+"/join", map[string]interface{}{
		"quantity": 2,
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	data := parseBody(w)["data"].(map[string]interface{})
	if data["status"] != "committed" {
		t.Errorf("expected committed, got %v", data["status"])
	}
}

func TestAdminCannotJoinGroupBuy(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doPost(env.Router, "/api/v1/group-buys/"+seedGroupBuyID+"/join", map[string]interface{}{
		"quantity": 1,
	}, token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDuplicateJoinPrevented(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	// First join
	doPost(env.Router, "/api/v1/group-buys/"+seedGroupBuyID+"/join", map[string]interface{}{
		"quantity": 1,
	}, token)

	// Second join should fail
	w := doPost(env.Router, "/api/v1/group-buys/"+seedGroupBuyID+"/join", map[string]interface{}{
		"quantity": 1,
	}, token)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate join, got %d: %s", w.Code, w.Body.String())
	}
}

func TestJoinTerminalStatePrevented(t *testing.T) {
	env := setupRouter(t)
	adminToken := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")
	memberToken := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	// Create and cancel a group buy
	cutoff := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	createW := doPost(env.Router, "/api/v1/group-buys", map[string]interface{}{
		"item_id": seedItemID, "location_id": seedLocationID,
		"title": "Cancel Test", "min_quantity": 5,
		"cutoff_at": cutoff, "price_per_unit": 10.00,
	}, adminToken)
	gbID := parseBody(createW)["data"].(map[string]interface{})["id"].(string)

	// Publish then cancel
	doPost(env.Router, "/api/v1/group-buys/"+gbID+"/publish", nil, adminToken)
	doPost(env.Router, "/api/v1/group-buys/"+gbID+"/cancel", nil, adminToken)

	// Try to join cancelled
	w := doPost(env.Router, "/api/v1/group-buys/"+gbID+"/join", nil, memberToken)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for cancelled GB, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublishAndCancelGroupBuy(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	cutoff := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	createW := doPost(env.Router, "/api/v1/group-buys", map[string]interface{}{
		"item_id": seedItemID, "location_id": seedLocationID,
		"title": "Publish Test", "min_quantity": 5,
		"cutoff_at": cutoff, "price_per_unit": 10.00,
	}, token)
	gbID := parseBody(createW)["data"].(map[string]interface{})["id"].(string)

	// Publish
	pubW := doPost(env.Router, "/api/v1/group-buys/"+gbID+"/publish", nil, token)
	if pubW.Code != http.StatusOK {
		t.Fatalf("publish: %d %s", pubW.Code, pubW.Body.String())
	}
	if parseBody(pubW)["data"].(map[string]interface{})["status"] != "published" {
		t.Error("expected published")
	}

	// Cancel
	cancelW := doPost(env.Router, "/api/v1/group-buys/"+gbID+"/cancel", nil, token)
	if cancelW.Code != http.StatusNoContent {
		t.Fatalf("cancel: %d %s", cancelW.Code, cancelW.Body.String())
	}
}

func TestGetParticipants(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/group-buys/"+seedGroupBuyID+"/participants", token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCoachCannotViewGroupBuys(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "coach@fitcommerce.dev", "Password123!")

	// Group-buy module is restricted to admin/operations/member.
	w := doGet(env.Router, "/api/v1/group-buys", token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestProcurementCannotViewGroupBuys(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "procurement@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/group-buys", token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCutoffAtMustBeFuture(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	past := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	w := doPost(env.Router, "/api/v1/group-buys", map[string]interface{}{
		"item_id": seedItemID, "location_id": seedLocationID,
		"title": "Past Cutoff", "min_quantity": 5,
		"cutoff_at": past, "price_per_unit": 10.00,
	}, token)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for past cutoff, got %d: %s", w.Code, w.Body.String())
	}
}
