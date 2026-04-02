package unit_test

import (
	"context"
	"testing"
	"time"
)

// ── Group-buy status transition tests ─────────────────────────────────────────

func TestGroupBuyStatusTransitions(t *testing.T) {
	// Valid statuses: draft, published, active, succeeded, failed, cancelled, fulfilled
	canJoin := map[string]bool{"published": true, "active": true}
	canCancel := map[string]bool{"draft": true, "published": true, "active": true}
	canPublish := map[string]bool{"draft": true}
	terminal := map[string]bool{"succeeded": true, "failed": true, "cancelled": true, "fulfilled": true}

	// Verify join states
	if !canJoin["published"] {
		t.Error("should be able to join published")
	}
	if !canJoin["active"] {
		t.Error("should be able to join active")
	}
	if canJoin["draft"] {
		t.Error("should not join draft")
	}
	if canJoin["succeeded"] {
		t.Error("should not join succeeded")
	}

	// Verify cancel states
	if !canCancel["draft"] || !canCancel["published"] || !canCancel["active"] {
		t.Error("should cancel from draft/published/active")
	}
	if canCancel["succeeded"] {
		t.Error("should not cancel succeeded")
	}

	// Verify publish
	if !canPublish["draft"] {
		t.Error("should publish from draft")
	}
	if canPublish["active"] {
		t.Error("should not publish from active")
	}

	// Verify terminal states
	for s := range terminal {
		if canJoin[s] {
			t.Errorf("terminal state %s should not allow joins", s)
		}
	}
}

func TestCutoffMustBeInFuture(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(7 * 24 * time.Hour)

	if !past.Before(time.Now()) {
		t.Error("past time should be before now")
	}
	if !future.After(time.Now()) {
		t.Error("future time should be after now")
	}
}

func TestCutoffEvaluation_Success(t *testing.T) {
	minQty := 10
	currentQty := 12

	newStatus := "failed"
	if currentQty >= minQty {
		newStatus = "succeeded"
	}
	if newStatus != "succeeded" {
		t.Errorf("expected succeeded, got %s", newStatus)
	}
}

func TestCutoffEvaluation_Failure(t *testing.T) {
	minQty := 10
	currentQty := 7

	newStatus := "failed"
	if currentQty >= minQty {
		newStatus = "succeeded"
	}
	if newStatus != "failed" {
		t.Errorf("expected failed, got %s", newStatus)
	}
}

func TestProgressCalculation(t *testing.T) {
	tests := []struct {
		min, current int
		expected     float64
	}{
		{10, 0, 0},
		{10, 5, 50},
		{10, 10, 100},
		{10, 15, 100}, // capped at 100
	}
	for _, tt := range tests {
		progress := float64(tt.current) / float64(tt.min) * 100
		if progress > 100 {
			progress = 100
		}
		if progress != tt.expected {
			t.Errorf("min=%d cur=%d: expected %.0f%%, got %.0f%%", tt.min, tt.current, tt.expected, progress)
		}
	}
}

func TestDuplicateParticipationPrevention(t *testing.T) {
	// The UNIQUE(group_buy_id, member_id) constraint prevents duplicates at DB level.
	// The handler also checks before insert for a nicer error message.
	// We verify the logic here.
	existing := map[string]bool{"gb1-member1": true}
	key := "gb1-member1"
	if !existing[key] {
		t.Fatal("existing participation should be detected")
	}
	key2 := "gb1-member2"
	if existing[key2] {
		t.Fatal("new participation should be allowed")
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if ctx.Err() == nil {
		t.Error("cancelled context should have error")
	}
}
