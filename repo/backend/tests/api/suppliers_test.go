package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSupplierAsAdmin(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doPost(env.Router, "/api/v1/suppliers", map[string]interface{}{
		"name":         "Acme Fitness Supply",
		"contact_name": "John Doe",
		"email":        "john@acme.com",
		"phone":        "555-0100",
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	data := parseBody(w)["data"].(map[string]interface{})
	if data["name"] != "Acme Fitness Supply" {
		t.Errorf("expected name Acme, got %v", data["name"])
	}
	if data["is_active"] != true {
		t.Error("default is_active should be true")
	}
}

func TestCreateSupplierAsMemberForbidden(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	w := doPost(env.Router, "/api/v1/suppliers", map[string]interface{}{
		"name": "Should Fail",
	}, token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCreateSupplierAsProcurement(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "procurement@fitcommerce.dev", "Password123!")

	w := doPost(env.Router, "/api/v1/suppliers", map[string]interface{}{
		"name": "Procurement Supplier",
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSupplier(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	// Create
	createW := doPost(env.Router, "/api/v1/suppliers", map[string]interface{}{
		"name": "Update Me",
	}, token)
	id := parseBody(createW)["data"].(map[string]interface{})["id"].(string)

	// Update
	body := map[string]interface{}{"name": "Updated Name", "is_active": false}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/suppliers/"+id, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	env.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := parseBody(w)["data"].(map[string]interface{})
	if data["name"] != "Updated Name" {
		t.Errorf("expected Updated Name, got %v", data["name"])
	}
	if data["is_active"] != false {
		t.Error("expected is_active = false")
	}
}

func TestListSuppliers(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/suppliers", token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSupplierValidation_MissingName(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doPost(env.Router, "/api/v1/suppliers", map[string]interface{}{}, token)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
