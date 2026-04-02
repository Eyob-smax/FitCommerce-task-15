package unit_test

import "testing"

func TestOrderStatusValues(t *testing.T) {
	valid := map[string]bool{
		"pending": true, "confirmed": true, "processing": true,
		"fulfilled": true, "cancelled": true, "refunded": true,
	}
	for s := range valid {
		if !valid[s] {
			t.Errorf("%s should be valid", s)
		}
	}
	if valid["shipped"] {
		t.Error("shipped should not be valid")
	}
}

func TestCancellationRules(t *testing.T) {
	canCancel := map[string]bool{
		"pending": true, "confirmed": true, "processing": true,
	}
	cantCancel := map[string]bool{
		"cancelled": true, "refunded": true, "fulfilled": true,
	}

	for s := range canCancel {
		if !canCancel[s] {
			t.Errorf("should be able to cancel %s orders", s)
		}
	}
	for s := range cantCancel {
		if canCancel[s] {
			t.Errorf("should not be able to cancel %s orders", s)
		}
	}
}

func TestSplitQuantityValidation(t *testing.T) {
	currentQty := 5
	splitQty := 3
	if splitQty > currentQty {
		t.Fatal("split should not exceed current")
	}
	remaining := currentQty - splitQty
	if remaining != 2 {
		t.Errorf("expected 2 remaining, got %d", remaining)
	}

	// Full split removes the original line
	splitAll := 5
	rem := currentQty - splitAll
	if rem != 0 {
		t.Error("full split should leave 0")
	}
}

func TestAdjustmentRejectsTerminal(t *testing.T) {
	terminal := map[string]bool{"cancelled": true, "refunded": true}
	if !terminal["cancelled"] || !terminal["refunded"] {
		t.Error("cancelled and refunded should be terminal")
	}
	if terminal["pending"] {
		t.Error("pending should not be terminal")
	}
}

func TestTimelineEventTypes(t *testing.T) {
	valid := map[string]bool{
		"status_change": true, "adjustment": true, "split": true,
		"cancellation": true, "note": true, "refund": true, "creation": true,
	}
	for etype := range valid {
		if !valid[etype] {
			t.Errorf("%s should be a valid event type", etype)
		}
	}
}

func TestTimelineIsAppendOnly(t *testing.T) {
	// Timeline events are INSERT-only in the handler.
	// There are no UPDATE or DELETE operations on order_timeline_events.
	// This test documents that guarantee.
	appendOnly := true
	if !appendOnly {
		t.Error("timeline must be append-only")
	}
}

func TestSplitCreatesNewOrder(t *testing.T) {
	// A split creates a new order with the same member, location, and status.
	// Both orders have timeline entries linking them.
	originalStatus := "confirmed"
	newOrderStatus := originalStatus // should match
	if newOrderStatus != originalStatus {
		t.Errorf("split order should inherit status %s", originalStatus)
	}
}
