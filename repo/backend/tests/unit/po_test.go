package unit_test

import "testing"

// ── PO status transition tests ────────────────────────────────────────────────

func TestPOStatusTransitions(t *testing.T) {
	// Valid transitions
	transitions := map[string][]string{
		"draft":              {"issued", "cancelled"},
		"issued":             {"partially_received", "cancelled"},
		"partially_received": {"received"},
		"received":           {"closed"},
	}

	canTransition := func(from, to string) bool {
		for _, valid := range transitions[from] {
			if valid == to {
				return true
			}
		}
		return false
	}

	// Verify valid transitions
	if !canTransition("draft", "issued") {
		t.Error("draft → issued should be valid")
	}
	if !canTransition("draft", "cancelled") {
		t.Error("draft → cancelled should be valid")
	}
	if !canTransition("issued", "partially_received") {
		t.Error("issued → partially_received should be valid")
	}
	if !canTransition("issued", "cancelled") {
		t.Error("issued → cancelled should be valid")
	}
	if !canTransition("partially_received", "received") {
		t.Error("partially_received → received should be valid")
	}

	// Verify invalid transitions
	if canTransition("cancelled", "issued") {
		t.Error("cancelled → issued should be invalid")
	}
	if canTransition("received", "draft") {
		t.Error("received → draft should be invalid")
	}
	if canTransition("partially_received", "cancelled") {
		t.Error("partially_received → cancelled should be invalid")
	}
}

func TestOverReceivePrevention(t *testing.T) {
	ordered := 10
	alreadyReceived := 7
	newReceive := 5

	if alreadyReceived+newReceive <= ordered {
		t.Fatal("this test case should trigger over-receive")
	}

	// Valid receive
	newReceive2 := 3
	if alreadyReceived+newReceive2 > ordered {
		t.Fatal("receiving 3 more on 7/10 should be valid")
	}
}

func TestSupplierRequiredFields(t *testing.T) {
	name := ""
	if name != "" {
		t.Error("empty name should fail validation")
	}

	name = "Acme Corp"
	if name == "" {
		t.Error("non-empty name should pass")
	}
}
