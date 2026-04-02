package api_test

import (
	"net/http"
	"testing"
)

func TestCreateItemRequiresAuth(t *testing.T) {
	env := setupRouter(t)

	w := doPost(env.Router, "/api/v1/items", map[string]interface{}{
		"name": "Test Item", "category": "gear", "price": 29.99,
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCreateItemAsAdmin(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doPost(env.Router, "/api/v1/items", map[string]interface{}{
		"name": "Yoga Mat", "category": "accessories", "price": 25.00,
		"condition": "new", "billing_model": "one-time",
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	body := parseBody(w)
	data := body["data"].(map[string]interface{})
	if data["name"] != "Yoga Mat" {
		t.Errorf("expected Yoga Mat, got %v", data["name"])
	}
	if data["status"] != "draft" {
		t.Errorf("new item should be draft, got %v", data["status"])
	}
	if data["deposit_amount"].(float64) != 50.0 {
		t.Errorf("default deposit should be 50, got %v", data["deposit_amount"])
	}
}

func TestCreateItemAsMemberForbidden(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	w := doPost(env.Router, "/api/v1/items", map[string]interface{}{
		"name": "Test", "category": "gear", "price": 10,
	}, token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestPublishAndUnpublishItem(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	// Create
	createW := doPost(env.Router, "/api/v1/items", map[string]interface{}{
		"name": "Band", "category": "gear", "price": 15.00,
	}, token)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", createW.Code, createW.Body.String())
	}
	id := parseBody(createW)["data"].(map[string]interface{})["id"].(string)

	// Publish
	pubW := doPost(env.Router, "/api/v1/items/"+id+"/publish", nil, token)
	if pubW.Code != http.StatusOK {
		t.Fatalf("publish: %d %s", pubW.Code, pubW.Body.String())
	}
	if parseBody(pubW)["data"].(map[string]interface{})["status"] != "published" {
		t.Error("expected published status")
	}

	// Cannot re-publish
	repub := doPost(env.Router, "/api/v1/items/"+id+"/publish", nil, token)
	if repub.Code != http.StatusConflict {
		t.Fatalf("re-publish should conflict, got %d", repub.Code)
	}

	// Unpublish
	unpubW := doPost(env.Router, "/api/v1/items/"+id+"/unpublish", nil, token)
	if unpubW.Code != http.StatusOK {
		t.Fatalf("unpublish: %d %s", unpubW.Code, unpubW.Body.String())
	}
	if parseBody(unpubW)["data"].(map[string]interface{})["status"] != "unpublished" {
		t.Error("expected unpublished status")
	}
}

func TestMemberCanOnlySeePublishedItems(t *testing.T) {
	env := setupRouter(t)
	adminToken := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")
	memberToken := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	// Create a draft item
	createW := doPost(env.Router, "/api/v1/items", map[string]interface{}{
		"name": "Hidden Item", "category": "gear", "price": 99.00,
	}, adminToken)
	id := parseBody(createW)["data"].(map[string]interface{})["id"].(string)

	// Member can't see draft
	getW := doGet(env.Router, "/api/v1/items/"+id, memberToken)
	if getW.Code != http.StatusNotFound {
		t.Fatalf("member should not see draft item, got %d", getW.Code)
	}

	// Publish it
	doPost(env.Router, "/api/v1/items/"+id+"/publish", nil, adminToken)

	// Now member can see it
	getW2 := doGet(env.Router, "/api/v1/items/"+id, memberToken)
	if getW2.Code != http.StatusOK {
		t.Fatalf("member should see published item, got %d", getW2.Code)
	}
}

func TestBatchUpdatePrice(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	// Create two items
	var ids []string
	for _, name := range []string{"Batch1", "Batch2"} {
		w := doPost(env.Router, "/api/v1/items", map[string]interface{}{
			"name": name, "category": "gear", "price": 10.00,
		}, token)
		ids = append(ids, parseBody(w)["data"].(map[string]interface{})["id"].(string))
	}

	// Batch update
	batchW := doPost(env.Router, "/api/v1/items/batch", map[string]interface{}{
		"item_ids": ids, "price": 19.99,
	}, token)
	if batchW.Code != http.StatusOK {
		t.Fatalf("batch: %d %s", batchW.Code, batchW.Body.String())
	}
	data := parseBody(batchW)["data"].(map[string]interface{})
	if data["updated"].(float64) != 2 {
		t.Errorf("expected 2 updated, got %v", data["updated"])
	}
}

func TestAvailabilityWindowCRUD(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	// Create item
	createW := doPost(env.Router, "/api/v1/items", map[string]interface{}{
		"name": "Window Test", "category": "gear", "price": 5.00,
	}, token)
	itemID := parseBody(createW)["data"].(map[string]interface{})["id"].(string)

	// Add window
	addW := doPost(env.Router, "/api/v1/items/"+itemID+"/availability-windows", map[string]string{
		"starts_at": "2025-07-01T09:00:00Z",
		"ends_at":   "2025-07-01T17:00:00Z",
	}, token)
	if addW.Code != http.StatusCreated {
		t.Fatalf("add window: %d %s", addW.Code, addW.Body.String())
	}

	// List windows
	listW := doGet(env.Router, "/api/v1/items/"+itemID+"/availability-windows", token)
	if listW.Code != http.StatusOK {
		t.Fatalf("list windows: %d", listW.Code)
	}

	// Invalid window (end before start)
	badW := doPost(env.Router, "/api/v1/items/"+itemID+"/availability-windows", map[string]string{
		"starts_at": "2025-07-01T17:00:00Z",
		"ends_at":   "2025-07-01T09:00:00Z",
	}, token)
	if badW.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid window should be 422, got %d", badW.Code)
	}
}
