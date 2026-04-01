package api_test

import (
	"net/http"
	"testing"
)

const seedMemberID = "33333333-0000-0000-0000-000000000001"

func createTestOrder(t *testing.T, env *testEnv, token string) string {
	t.Helper()
	w := doPost(env.Router, "/api/v1/orders", map[string]interface{}{
		"member_id":    seedMemberID,
		"location_id":  seedLocationID,
		"total_amount": 49.98,
		"lines": []map[string]interface{}{
			{"item_id": seedItemID, "quantity": 2, "unit_price": 24.99},
		},
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("create order: %d %s", w.Code, w.Body.String())
	}
	return parseBody(w)["data"].(map[string]interface{})["id"].(string)
}

func TestCreateOrder(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	orderID := createTestOrder(t, env, token)
	if orderID == "" {
		t.Fatal("order ID should not be empty")
	}

	// Get order
	w := doGet(env.Router, "/api/v1/orders/"+orderID, token)
	if w.Code != http.StatusOK {
		t.Fatalf("get order: %d", w.Code)
	}
	data := parseBody(w)["data"].(map[string]interface{})
	if data["status"] != "pending" {
		t.Errorf("expected pending, got %v", data["status"])
	}
}

func TestOrderTimeline(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	orderID := createTestOrder(t, env, token)

	// Timeline should have creation event
	w := doGet(env.Router, "/api/v1/orders/"+orderID+"/timeline", token)
	if w.Code != http.StatusOK {
		t.Fatalf("timeline: %d", w.Code)
	}
	data := parseBody(w)["data"].([]interface{})
	if len(data) == 0 {
		t.Error("timeline should have at least one event")
	}
	first := data[0].(map[string]interface{})
	if first["event_type"] != "creation" {
		t.Errorf("first event should be creation, got %v", first["event_type"])
	}
}

func TestAddNoteCreatesTimelineEvent(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	orderID := createTestOrder(t, env, token)

	// Add note
	noteW := doPost(env.Router, "/api/v1/orders/"+orderID+"/notes", map[string]string{
		"content": "Customer called about delivery",
	}, token)
	if noteW.Code != http.StatusCreated {
		t.Fatalf("add note: %d %s", noteW.Code, noteW.Body.String())
	}

	// Timeline should now have 2 events: creation + note
	tlW := doGet(env.Router, "/api/v1/orders/"+orderID+"/timeline", token)
	events := parseBody(tlW)["data"].([]interface{})
	if len(events) < 2 {
		t.Errorf("expected at least 2 timeline events, got %d", len(events))
	}
}

func TestCancelOrder(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	orderID := createTestOrder(t, env, token)

	cancelW := doPost(env.Router, "/api/v1/orders/"+orderID+"/cancel", map[string]string{
		"reason": "Customer requested cancellation",
	}, token)
	if cancelW.Code != http.StatusNoContent {
		t.Fatalf("cancel: %d %s", cancelW.Code, cancelW.Body.String())
	}

	// Cannot cancel again
	recancel := doPost(env.Router, "/api/v1/orders/"+orderID+"/cancel", nil, token)
	if recancel.Code != http.StatusConflict {
		t.Fatalf("re-cancel should 409, got %d", recancel.Code)
	}
}

func TestAdjustOrderLine(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	orderID := createTestOrder(t, env, token)

	// Get the line ID
	getW := doGet(env.Router, "/api/v1/orders/"+orderID, token)
	lines := parseBody(getW)["data"].(map[string]interface{})["lines"].([]interface{})
	lineID := lines[0].(map[string]interface{})["id"].(string)

	// Adjust
	adjW := doPost(env.Router, "/api/v1/orders/"+orderID+"/adjust", map[string]interface{}{
		"line_id":      lineID,
		"new_quantity": 5,
		"reason":       "Customer wants more",
	}, token)
	if adjW.Code != http.StatusOK {
		t.Fatalf("adjust: %d %s", adjW.Code, adjW.Body.String())
	}

	// Total should be recalculated
	data := parseBody(adjW)["data"].(map[string]interface{})
	newTotal := data["total_amount"].(float64)
	expected := 5 * 24.99
	if newTotal != expected {
		t.Errorf("expected total %.2f, got %.2f", expected, newTotal)
	}
}

func TestSplitOrder(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	// Create order with 5 qty
	w := doPost(env.Router, "/api/v1/orders", map[string]interface{}{
		"member_id": seedMemberID, "location_id": seedLocationID,
		"total_amount": 124.95,
		"lines": []map[string]interface{}{
			{"item_id": seedItemID, "quantity": 5, "unit_price": 24.99},
		},
	}, token)
	orderID := parseBody(w)["data"].(map[string]interface{})["id"].(string)
	lineID := parseBody(w)["data"].(map[string]interface{})["lines"].([]interface{})[0].(map[string]interface{})["id"].(string)

	// Split 2 units to new order
	splitW := doPost(env.Router, "/api/v1/orders/"+orderID+"/split", map[string]interface{}{
		"lines":  []map[string]interface{}{{"line_id": lineID, "quantity": 2}},
		"reason": "Partial shipment",
	}, token)
	if splitW.Code != http.StatusCreated {
		t.Fatalf("split: %d %s", splitW.Code, splitW.Body.String())
	}
	splitData := parseBody(splitW)["data"].(map[string]interface{})
	newOrderID := splitData["new_order_id"].(string)
	if newOrderID == "" {
		t.Error("new_order_id should not be empty")
	}

	// Original should have 3 remaining
	origW := doGet(env.Router, "/api/v1/orders/"+orderID, token)
	origLines := parseBody(origW)["data"].(map[string]interface{})["lines"].([]interface{})
	origQty := origLines[0].(map[string]interface{})["quantity"].(float64)
	if origQty != 3 {
		t.Errorf("original should have 3, got %.0f", origQty)
	}

	// New order should have 2
	newW := doGet(env.Router, "/api/v1/orders/"+newOrderID, token)
	newLines := parseBody(newW)["data"].(map[string]interface{})["lines"].([]interface{})
	newQty := newLines[0].(map[string]interface{})["quantity"].(float64)
	if newQty != 2 {
		t.Errorf("split order should have 2, got %.0f", newQty)
	}
}

func TestMemberCanOnlySeeOwnOrders(t *testing.T) {
	env := setupRouter(t)
	adminToken := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")
	memberToken := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	// Create order for the member
	orderID := createTestOrder(t, env, adminToken)

	// Member should see it (it's their member_id)
	w := doGet(env.Router, "/api/v1/orders/"+orderID, memberToken)
	if w.Code != http.StatusOK {
		t.Fatalf("member should see own order, got %d", w.Code)
	}
}

func TestMemberCannotAdjustOrder(t *testing.T) {
	env := setupRouter(t)
	adminToken := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")
	memberToken := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	orderID := createTestOrder(t, env, adminToken)

	w := doPost(env.Router, "/api/v1/orders/"+orderID+"/adjust", map[string]interface{}{
		"line_id": "fake", "new_quantity": 1, "reason": "test",
	}, memberToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("member should not adjust, got %d", w.Code)
	}
}

func TestMemberCannotAddOrderNote(t *testing.T) {
	env := setupRouter(t)
	adminToken := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")
	memberToken := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	orderID := createTestOrder(t, env, adminToken)

	w := doPost(env.Router, "/api/v1/orders/"+orderID+"/notes", map[string]string{
		"content": "member note",
	}, memberToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("member should not add notes, got %d", w.Code)
	}
}

func TestSplitExceedsQuantity(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doPost(env.Router, "/api/v1/orders", map[string]interface{}{
		"member_id": seedMemberID, "location_id": seedLocationID,
		"total_amount": 24.99,
		"lines": []map[string]interface{}{
			{"item_id": seedItemID, "quantity": 1, "unit_price": 24.99},
		},
	}, token)
	orderID := parseBody(w)["data"].(map[string]interface{})["id"].(string)
	lineID := parseBody(w)["data"].(map[string]interface{})["lines"].([]interface{})[0].(map[string]interface{})["id"].(string)

	splitW := doPost(env.Router, "/api/v1/orders/"+orderID+"/split", map[string]interface{}{
		"lines":  []map[string]interface{}{{"line_id": lineID, "quantity": 5}},
		"reason": "Too many",
	}, token)
	if splitW.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for over-split, got %d: %s", splitW.Code, splitW.Body.String())
	}
}
