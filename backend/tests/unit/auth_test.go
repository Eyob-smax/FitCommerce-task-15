package unit_test

import (
	"testing"

	"fitcommerce/backend/internal/auth"
)

// ── Password hashing ──────────────────────────────────────────────────────────

func TestHashPasswordAndCheck(t *testing.T) {
	plain := "S3cret!Pass"
	hash, err := auth.HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == plain {
		t.Fatal("hash should differ from plaintext")
	}
	if !auth.CheckPassword(plain, hash) {
		t.Fatal("CheckPassword should return true for correct password")
	}
	if auth.CheckPassword("wrong", hash) {
		t.Fatal("CheckPassword should return false for wrong password")
	}
}

// ── JWT issue / validate ──────────────────────────────────────────────────────

func newTestManager() *auth.Manager {
	cfg := auth.TestJWTConfig()
	return auth.NewManager(&cfg)
}

func TestJWTIssueAndValidateAccess(t *testing.T) {
	mgr := newTestManager()

	pair, err := mgr.Issue("uid-1", "admin@test.com", auth.RoleAdministrator)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("tokens must not be empty")
	}
	if pair.ExpiresIn <= 0 {
		t.Fatal("expires_in must be positive")
	}

	claims, err := mgr.Validate(pair.AccessToken)
	if err != nil {
		t.Fatalf("Validate access: %v", err)
	}
	if claims.UserID != "uid-1" {
		t.Errorf("expected uid-1, got %s", claims.UserID)
	}
	if claims.Email != "admin@test.com" {
		t.Errorf("expected admin@test.com, got %s", claims.Email)
	}
	if claims.Role != auth.RoleAdministrator {
		t.Errorf("expected administrator, got %s", claims.Role)
	}
}

func TestJWTValidateRefreshToken(t *testing.T) {
	mgr := newTestManager()

	pair, err := mgr.Issue("uid-2", "user@test.com", auth.RoleMember)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	claims, err := mgr.ValidateRefresh(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateRefresh: %v", err)
	}
	if claims.UserID != "uid-2" {
		t.Errorf("expected uid-2, got %s", claims.UserID)
	}
}

func TestJWTAccessTokenCannotBeUsedAsRefresh(t *testing.T) {
	mgr := newTestManager()
	pair, _ := mgr.Issue("uid-3", "a@b.com", auth.RoleMember)

	_, err := mgr.ValidateRefresh(pair.AccessToken)
	if err == nil {
		t.Fatal("access token should not validate as refresh token")
	}
}

func TestJWTRefreshTokenCannotBeUsedAsAccess(t *testing.T) {
	mgr := newTestManager()
	pair, _ := mgr.Issue("uid-4", "a@b.com", auth.RoleMember)

	_, err := mgr.Validate(pair.RefreshToken)
	if err == nil {
		t.Fatal("refresh token should not validate as access token")
	}
}

func TestJWTInvalidTokenFails(t *testing.T) {
	mgr := newTestManager()
	_, err := mgr.Validate("not.a.valid.token")
	if err == nil {
		t.Fatal("should reject garbage token")
	}
}

// ── Permission matrix ─────────────────────────────────────────────────────────

func TestAdminHasAllCorePermissions(t *testing.T) {
	perms := []auth.Permission{
		auth.PermSystemConfig, auth.PermUserManage,
		auth.PermCatalogRead, auth.PermCatalogWrite,
		auth.PermInventoryRead, auth.PermInventoryAdjust,
		auth.PermSupplierRead, auth.PermSupplierWrite,
		auth.PermPORead, auth.PermPOWrite, auth.PermPOReceive,
		auth.PermAuditRead,
	}
	for _, p := range perms {
		if !auth.HasPermission(auth.RoleAdministrator, p) {
			t.Errorf("admin should have %s", p)
		}
	}
}

func TestCoachCannotAccessSupplierOrSystem(t *testing.T) {
	forbidden := []auth.Permission{
		auth.PermSystemConfig, auth.PermUserManage,
		auth.PermSupplierRead, auth.PermSupplierWrite,
		auth.PermPORead, auth.PermPOWrite, auth.PermPOReceive,
	}
	for _, p := range forbidden {
		if auth.HasPermission(auth.RoleCoach, p) {
			t.Errorf("coach should NOT have %s", p)
		}
	}
}

func TestMemberCannotAccessSupplierOrSystem(t *testing.T) {
	forbidden := []auth.Permission{
		auth.PermSystemConfig, auth.PermUserManage,
		auth.PermSupplierRead, auth.PermSupplierWrite,
		auth.PermPORead, auth.PermPOWrite, auth.PermPOReceive,
		auth.PermInventoryAdjust,
	}
	for _, p := range forbidden {
		if auth.HasPermission(auth.RoleMember, p) {
			t.Errorf("member should NOT have %s", p)
		}
	}
}

func TestMemberHasOwnPermissions(t *testing.T) {
	allowed := []auth.Permission{
		auth.PermCatalogRead,
		auth.PermGroupBuyRead, auth.PermGroupBuyCreate, auth.PermGroupBuyJoin,
		auth.PermOrderOwnRead,
		auth.PermMemberBrowse,
	}
	for _, p := range allowed {
		if !auth.HasPermission(auth.RoleMember, p) {
			t.Errorf("member should have %s", p)
		}
	}
}

func TestUnknownRoleHasNoPermissions(t *testing.T) {
	if auth.HasPermission("ghost", auth.PermCatalogRead) {
		t.Error("unknown role should have no permissions")
	}
}

func TestGetPermissionsReturnsNonEmpty(t *testing.T) {
	for _, role := range []string{
		auth.RoleAdministrator,
		auth.RoleOperationsManager,
		auth.RoleProcurementSpecialist,
		auth.RoleCoach,
		auth.RoleMember,
	} {
		perms := auth.GetPermissions(role)
		if len(perms) == 0 {
			t.Errorf("%s should have at least one permission", role)
		}
	}
}
