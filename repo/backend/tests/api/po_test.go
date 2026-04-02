package api_test

import (
	"net/http"
	"testing"
)

// These tests require seeded suppliers, items, and locations.
// The seeder creates fixed UUIDs for these (see seeds/seeder.go).

const (
	seedSupplierID = "55555555-0000-0000-0000-000000000001"
	seedLocationID = "11111111-0000-0000-0000-000000000001"
	seedItemID     = "66666666-0000-0000-0000-000000000001"
)

func TestCreatePOAndIssue(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	// Create PO
	createW := doPost(env.Router, "/api/v1/purchase-orders", map[string]interface{}{
		"supplier_id": seedSupplierID,
		"location_id": seedLocationID,
		"notes":       "Test PO",
		"lines": []map[string]interface{}{
			{"item_id": seedItemID, "quantity": 10, "unit_cost": 5.00},
		},
	}, token)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createW.Code, createW.Body.String())
	}
	data := parseBody(createW)["data"].(map[string]interface{})
	poID := data["id"].(string)
	if data["status"] != "draft" {
		t.Errorf("expected draft, got %v", data["status"])
	}

	// Issue
	issueW := doPost(env.Router, "/api/v1/purchase-orders/"+poID+"/issue", nil, token)
	if issueW.Code != http.StatusOK {
		t.Fatalf("issue: %d %s", issueW.Code, issueW.Body.String())
	}
	issueData := parseBody(issueW)["data"].(map[string]interface{})
	if issueData["status"] != "issued" {
		t.Errorf("expected issued, got %v", issueData["status"])
	}
}

func TestCannotIssueNonDraftPO(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	// Create and issue
	createW := doPost(env.Router, "/api/v1/purchase-orders", map[string]interface{}{
		"supplier_id": seedSupplierID,
		"location_id": seedLocationID,
		"lines":       []map[string]interface{}{{"item_id": seedItemID, "quantity": 5, "unit_cost": 3.00}},
	}, token)
	poID := parseBody(createW)["data"].(map[string]interface{})["id"].(string)
	doPost(env.Router, "/api/v1/purchase-orders/"+poID+"/issue", nil, token)

	// Try to issue again
	reIssue := doPost(env.Router, "/api/v1/purchase-orders/"+poID+"/issue", nil, token)
	if reIssue.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", reIssue.Code)
	}
}

func TestCancelDraftPO(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	createW := doPost(env.Router, "/api/v1/purchase-orders", map[string]interface{}{
		"supplier_id": seedSupplierID,
		"location_id": seedLocationID,
		"lines":       []map[string]interface{}{{"item_id": seedItemID, "quantity": 5, "unit_cost": 3.00}},
	}, token)
	poID := parseBody(createW)["data"].(map[string]interface{})["id"].(string)

	cancelW := doPost(env.Router, "/api/v1/purchase-orders/"+poID+"/cancel", nil, token)
	if cancelW.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", cancelW.Code, cancelW.Body.String())
	}
}

func TestReceivePOUpdatesStock(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	// Create and issue PO
	createW := doPost(env.Router, "/api/v1/purchase-orders", map[string]interface{}{
		"supplier_id": seedSupplierID,
		"location_id": seedLocationID,
		"lines":       []map[string]interface{}{{"item_id": seedItemID, "quantity": 20, "unit_cost": 2.50}},
	}, token)
	data := parseBody(createW)["data"].(map[string]interface{})
	poID := data["id"].(string)
	lines := data["lines"].([]interface{})
	lineID := lines[0].(map[string]interface{})["id"].(string)

	doPost(env.Router, "/api/v1/purchase-orders/"+poID+"/issue", nil, token)

	// Partial receive
	receiveW := doPost(env.Router, "/api/v1/purchase-orders/"+poID+"/receive", map[string]interface{}{
		"notes": "Partial delivery",
		"lines": []map[string]interface{}{
			{"po_line_item_id": lineID, "quantity_received": 12},
		},
	}, token)
	if receiveW.Code != http.StatusCreated {
		t.Fatalf("receive: %d %s", receiveW.Code, receiveW.Body.String())
	}

	// Check PO is partially_received
	getW := doGet(env.Router, "/api/v1/purchase-orders/"+poID, token)
	poData := parseBody(getW)["data"].(map[string]interface{})
	if poData["status"] != "partially_received" {
		t.Errorf("expected partially_received, got %v", poData["status"])
	}

	// Full receive remaining
	receiveW2 := doPost(env.Router, "/api/v1/purchase-orders/"+poID+"/receive", map[string]interface{}{
		"lines": []map[string]interface{}{
			{"po_line_item_id": lineID, "quantity_received": 8},
		},
	}, token)
	if receiveW2.Code != http.StatusCreated {
		t.Fatalf("receive2: %d %s", receiveW2.Code, receiveW2.Body.String())
	}

	// Check PO is fully received
	getW2 := doGet(env.Router, "/api/v1/purchase-orders/"+poID, token)
	poData2 := parseBody(getW2)["data"].(map[string]interface{})
	if poData2["status"] != "received" {
		t.Errorf("expected received, got %v", poData2["status"])
	}
}

func TestOverReceivePrevented(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	createW := doPost(env.Router, "/api/v1/purchase-orders", map[string]interface{}{
		"supplier_id": seedSupplierID,
		"location_id": seedLocationID,
		"lines":       []map[string]interface{}{{"item_id": seedItemID, "quantity": 5, "unit_cost": 1.00}},
	}, token)
	data := parseBody(createW)["data"].(map[string]interface{})
	poID := data["id"].(string)
	lineID := data["lines"].([]interface{})[0].(map[string]interface{})["id"].(string)

	doPost(env.Router, "/api/v1/purchase-orders/"+poID+"/issue", nil, token)

	// Try to receive more than ordered
	overW := doPost(env.Router, "/api/v1/purchase-orders/"+poID+"/receive", map[string]interface{}{
		"lines": []map[string]interface{}{
			{"po_line_item_id": lineID, "quantity_received": 10},
		},
	}, token)
	if overW.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for over-receive, got %d: %s", overW.Code, overW.Body.String())
	}
}

func TestMemberCannotAccessPOs(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/purchase-orders", token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCoachCannotAccessSuppliers(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "coach@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/suppliers", token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
