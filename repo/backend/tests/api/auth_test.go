package api_test

import (
	"net/http"
	"testing"
)

// ── Login ─────────────────────────────────────────────────────────────────────

func TestLoginSuccess(t *testing.T) {
	env := setupRouter(t)

	w := doPost(env.Router, "/api/v1/auth/login", map[string]string{
		"email":    "admin@fitcommerce.dev",
		"password": "Password123!",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := parseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing data object")
	}
	if data["access_token"] == nil || data["access_token"] == "" {
		t.Error("missing access_token")
	}
	if data["refresh_token"] == nil || data["refresh_token"] == "" {
		t.Error("missing refresh_token")
	}
	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatal("missing user in login response")
	}
	if user["role"] != "administrator" {
		t.Errorf("expected role=administrator, got %v", user["role"])
	}
}

func TestLoginWrongPassword(t *testing.T) {
	env := setupRouter(t)

	w := doPost(env.Router, "/api/v1/auth/login", map[string]string{
		"email":    "admin@fitcommerce.dev",
		"password": "wrong",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLoginNonexistentEmail(t *testing.T) {
	env := setupRouter(t)

	w := doPost(env.Router, "/api/v1/auth/login", map[string]string{
		"email":    "nobody@fitcommerce.com",
		"password": "any",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLoginMissingFields(t *testing.T) {
	env := setupRouter(t)

	w := doPost(env.Router, "/api/v1/auth/login", map[string]string{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ── Refresh ───────────────────────────────────────────────────────────────────

func TestRefreshTokenRotation(t *testing.T) {
	env := setupRouter(t)

	// Login first.
	loginW := doPost(env.Router, "/api/v1/auth/login", map[string]string{
		"email":    "admin@fitcommerce.dev",
		"password": "Password123!",
	})
	if loginW.Code != http.StatusOK {
		t.Fatalf("login failed: %d", loginW.Code)
	}
	loginData := parseBody(loginW)["data"].(map[string]interface{})
	refreshToken := loginData["refresh_token"].(string)

	// Refresh.
	refreshW := doPost(env.Router, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	})
	if refreshW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", refreshW.Code, refreshW.Body.String())
	}
	refreshData := parseBody(refreshW)["data"].(map[string]interface{})
	if refreshData["access_token"] == nil || refreshData["access_token"] == "" {
		t.Error("missing new access_token")
	}
	newRefresh := refreshData["refresh_token"].(string)
	if newRefresh == refreshToken {
		t.Error("refresh token should rotate (new != old)")
	}

	// Old token should now be revoked.
	replayW := doPost(env.Router, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	})
	if replayW.Code != http.StatusUnauthorized {
		t.Fatalf("replayed old refresh token should be 401, got %d", replayW.Code)
	}
}

func TestRefreshWithInvalidToken(t *testing.T) {
	env := setupRouter(t)

	w := doPost(env.Router, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": "garbage.token.value",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// ── /users/me ─────────────────────────────────────────────────────────────────

func TestMeReturnsCurrentUser(t *testing.T) {
	env := setupRouter(t)

	loginW := doPost(env.Router, "/api/v1/auth/login", map[string]string{
		"email":    "admin@fitcommerce.dev",
		"password": "Password123!",
	})
	data := parseBody(loginW)["data"].(map[string]interface{})
	accessToken := data["access_token"].(string)

	meW := doGet(env.Router, "/api/v1/users/me", accessToken)
	if meW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", meW.Code, meW.Body.String())
	}
	meData := parseBody(meW)["data"].(map[string]interface{})
	if meData["email"] != "admin@fitcommerce.dev" {
		t.Errorf("expected admin email, got %v", meData["email"])
	}
}

func TestMeWithoutTokenIs401(t *testing.T) {
	env := setupRouter(t)

	w := doGet(env.Router, "/api/v1/users/me", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// ── Logout ────────────────────────────────────────────────────────────────────

func TestLogoutRevokesToken(t *testing.T) {
	env := setupRouter(t)

	loginW := doPost(env.Router, "/api/v1/auth/login", map[string]string{
		"email":    "admin@fitcommerce.dev",
		"password": "Password123!",
	})
	data := parseBody(loginW)["data"].(map[string]interface{})
	accessToken := data["access_token"].(string)
	refreshToken := data["refresh_token"].(string)

	logoutW := doPost(env.Router, "/api/v1/auth/logout", map[string]string{
		"refresh_token": refreshToken,
	}, accessToken)
	if logoutW.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", logoutW.Code, logoutW.Body.String())
	}

	// Refresh with the revoked token should fail.
	refreshW := doPost(env.Router, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	})
	if refreshW.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", refreshW.Code)
	}
}

// ── RBAC enforcement ──────────────────────────────────────────────────────────

func loginAs(t *testing.T, env *testEnv, email, password string) string {
	t.Helper()
	w := doPost(env.Router, "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("login as %s failed: %d %s", email, w.Code, w.Body.String())
	}
	data := parseBody(w)["data"].(map[string]interface{})
	return data["access_token"].(string)
}

func TestProtectedEndpointRequiresAuth(t *testing.T) {
	env := setupRouter(t)

	w := doGet(env.Router, "/api/v1/users", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// ── Health check ──────────────────────────────────────────────────────────────

func TestHealthEndpoint(t *testing.T) {
	env := setupRouter(t)

	w := doGet(env.Router, "/health", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
