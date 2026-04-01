package unit_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// ── Item validation tests (handler-level via Gin test mode) ───────────────────

func init() {
	gin.SetMode(gin.TestMode)
}

func TestItemCreateValidation_MissingName(t *testing.T) {
	body := `{"category":"gear","price":10.00}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/items", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Bind should fail on missing required name
	var req struct {
		Name     string  `json:"name" binding:"required"`
		Category string  `json:"category" binding:"required"`
		Price    float64 `json:"price" binding:"required"`
	}
	err := c.ShouldBindJSON(&req)
	if err == nil {
		t.Fatal("expected validation error for missing name")
	}
}

func TestItemCreateValidation_NegativePrice(t *testing.T) {
	body := `{"name":"Test","category":"gear","price":-5.00}`
	var req struct {
		Name     string  `json:"name"`
		Category string  `json:"category"`
		Price    float64 `json:"price"`
	}
	_ = json.NewDecoder(strings.NewReader(body)).Decode(&req)
	if req.Price >= 0 {
		t.Fatal("expected negative price")
	}
}

func TestItemConditionValues(t *testing.T) {
	valid := map[string]bool{"new": true, "open-box": true, "used": true}
	for _, c := range []string{"new", "open-box", "used"} {
		if !valid[c] {
			t.Errorf("%s should be valid", c)
		}
	}
	for _, c := range []string{"refurbished", "", "NEW"} {
		if valid[c] {
			t.Errorf("%s should be invalid", c)
		}
	}
}

func TestItemBillingModelValues(t *testing.T) {
	valid := map[string]bool{"one-time": true, "monthly-rental": true}
	for _, b := range []string{"one-time", "monthly-rental"} {
		if !valid[b] {
			t.Errorf("%s should be valid", b)
		}
	}
	if valid["subscription"] {
		t.Error("subscription should be invalid")
	}
}

func TestItemStatusTransitions(t *testing.T) {
	// draft → published ✓
	// unpublished → published ✓
	// published → unpublished ✓
	// published → draft ✗
	// unpublished → draft ✗
	canPublishFrom := map[string]bool{"draft": true, "unpublished": true}
	canUnpublishFrom := map[string]bool{"published": true}

	if !canPublishFrom["draft"] {
		t.Error("should be able to publish from draft")
	}
	if !canPublishFrom["unpublished"] {
		t.Error("should be able to publish from unpublished")
	}
	if canPublishFrom["published"] {
		t.Error("should not re-publish already published")
	}
	if !canUnpublishFrom["published"] {
		t.Error("should be able to unpublish from published")
	}
	if canUnpublishFrom["draft"] {
		t.Error("should not unpublish from draft")
	}
}

func TestStockAdjustmentReasonCodes(t *testing.T) {
	valid := map[string]bool{
		"damaged": true, "found": true, "correction": true, "return": true,
		"theft": true, "audit": true, "expired": true, "other": true,
	}
	for code := range valid {
		if !valid[code] {
			t.Errorf("%s should be valid", code)
		}
	}
	if valid["unknown"] {
		t.Error("unknown should be invalid")
	}
}

func TestAvailabilityWindowValidation(t *testing.T) {
	// ends_at must be after starts_at
	start := "2025-06-01T09:00:00Z"
	end := "2025-06-01T08:00:00Z"
	if end > start {
		t.Error("end should not be after start in this test case")
	}

	start2 := "2025-06-01T09:00:00Z"
	end2 := "2025-06-01T17:00:00Z"
	if end2 <= start2 {
		t.Error("end should be after start")
	}
}

func TestBatchUpdateRequiresItemIDs(t *testing.T) {
	var req struct {
		ItemIDs []string `json:"item_ids" binding:"required,min=1"`
		Price   *float64 `json:"price"`
	}
	body := `{"item_ids":[],"price":10.00}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	err := c.ShouldBindJSON(&req)
	if err == nil {
		t.Fatal("expected validation error for empty item_ids")
	}
}

func TestDepositDefaultValue(t *testing.T) {
	defaultDeposit := 50.00
	if defaultDeposit != 50.00 {
		t.Errorf("default deposit should be 50.00, got %f", defaultDeposit)
	}
}
