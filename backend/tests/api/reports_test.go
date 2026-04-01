package api_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDashboardKPI(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/reports/dashboard?granularity=monthly", token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := parseBody(w)["data"].(map[string]interface{})
	requiredFields := []string{"member_growth", "member_churn", "renewal_rate", "engagement", "class_fill_rate", "coach_productivity"}
	for _, f := range requiredFields {
		if _, ok := data[f]; !ok {
			t.Errorf("missing KPI field: %s", f)
		}
	}
}

func TestDashboardWithGranularity(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	for _, g := range []string{"daily", "weekly", "monthly"} {
		w := doGet(env.Router, "/api/v1/reports/dashboard?granularity="+g, token)
		if w.Code != http.StatusOK {
			t.Fatalf("granularity %s: %d", g, w.Code)
		}
	}
}

func TestDashboardWithDateRangeAndItemCategory(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/reports/dashboard?granularity=monthly&item_category=gear&start_date=2026-01-01&end_date=2026-01-31", token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := parseBody(w)["data"].(map[string]interface{})
	if data["start_date"] != "2026-01-01" {
		t.Fatalf("expected start_date 2026-01-01, got %v", data["start_date"])
	}
	if data["end_date"] != "2026-01-31" {
		t.Fatalf("expected end_date 2026-01-31, got %v", data["end_date"])
	}
}

func TestDashboardMemberForbidden(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/reports/dashboard", token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("member should not access dashboard, got %d", w.Code)
	}
}

func TestCoachSeesOwnReport(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "coach@fitcommerce.dev", "Password123!")
	coachID := "44444444-0000-0000-0000-000000000001"

	w := doGet(env.Router, "/api/v1/reports/coach/"+coachID, token)
	if w.Code != http.StatusOK {
		t.Fatalf("coach should see own report, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCoachCannotSeeOtherCoachReport(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "coach@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/reports/coach/00000000-0000-0000-0000-000000000099", token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("coach should not see other coach report, got %d", w.Code)
	}
}

func TestMemberCannotAccessCoachReport(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/reports/coach/44444444-0000-0000-0000-000000000001", token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("member should not access coach report, got %d", w.Code)
	}
}

func TestProcurementCannotAccessCoachReport(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "procurement@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/reports/coach/44444444-0000-0000-0000-000000000001", token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("procurement should not access coach report, got %d", w.Code)
	}
}

func TestMemberGrowthReport(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/reports/member-growth", token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestInventoryReport(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/reports/inventory", token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCreateExportCSV(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doPost(env.Router, "/api/v1/exports", map[string]interface{}{
		"report_type": "inventory",
		"format":      "csv",
		"filters":     map[string]string{},
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	data := parseBody(w)["data"].(map[string]interface{})
	if data["status"] != "queued" {
		t.Errorf("expected queued, got %v", data["status"])
	}
}

func TestCreateExportPDF(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doPost(env.Router, "/api/v1/exports", map[string]interface{}{
		"report_type": "inventory",
		"format":      "pdf",
		"filters":     map[string]string{},
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExportInvalidFormat(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doPost(env.Router, "/api/v1/exports", map[string]interface{}{
		"report_type": "inventory",
		"format":      "xlsx",
	}, token)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid format, got %d", w.Code)
	}
}

func TestMemberCannotExport(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "member@fitcommerce.dev", "Password123!")

	w := doPost(env.Router, "/api/v1/exports", map[string]interface{}{
		"report_type": "inventory", "format": "csv",
	}, token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("member should not export, got %d", w.Code)
	}
}

func TestListExports(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	w := doGet(env.Router, "/api/v1/exports", token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestExportDownloadRequiresAuth(t *testing.T) {
	env := setupRouter(t)

	w := doGet(env.Router, "/api/v1/exports/11111111-1111-1111-1111-111111111111/download", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestExportDownloadCompletedCSV(t *testing.T) {
	env := setupRouter(t)
	token := loginAs(t, env, "admin@fitcommerce.dev", "Password123!")

	createW := doPost(env.Router, "/api/v1/exports", map[string]interface{}{
		"report_type": "inventory",
		"format":      "csv",
		"filters":     map[string]string{},
	}, token)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createW.Code, createW.Body.String())
	}
	jobID := parseBody(createW)["data"].(map[string]interface{})["id"].(string)

	deadline := time.Now().Add(5 * time.Second)
	status := ""
	for time.Now().Before(deadline) {
		statusW := doGet(env.Router, "/api/v1/exports/"+jobID, token)
		if statusW.Code != http.StatusOK {
			t.Fatalf("expected 200 when fetching job, got %d", statusW.Code)
		}
		status = parseBody(statusW)["data"].(map[string]interface{})["status"].(string)
		if status == "completed" {
			break
		}
		if status == "failed" {
			t.Fatalf("export failed unexpectedly: %s", statusW.Body.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
	if status != "completed" {
		t.Fatalf("export did not complete within timeout, status=%s", status)
	}

	downloadW := doGet(env.Router, "/api/v1/exports/"+jobID+"/download", token)
	if downloadW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", downloadW.Code, downloadW.Body.String())
	}
	if got := downloadW.Header().Get("Content-Disposition"); got == "" || !strings.Contains(got, ".csv") {
		t.Fatalf("expected csv content-disposition, got %q", got)
	}
}
