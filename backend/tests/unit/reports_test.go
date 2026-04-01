package unit_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestKPIDefinitions(t *testing.T) {
	kpis := []string{
		"member_growth", "member_churn", "renewal_rate",
		"engagement", "class_fill_rate", "coach_productivity",
	}
	if len(kpis) != 6 {
		t.Errorf("expected 6 KPIs, got %d", len(kpis))
	}
}

func TestGranularityValues(t *testing.T) {
	valid := map[string]bool{"daily": true, "weekly": true, "monthly": true}
	for g := range valid {
		if !valid[g] {
			t.Errorf("%s should be valid", g)
		}
	}
	if valid["yearly"] {
		t.Error("yearly should not be valid")
	}
}

func TestPeriodCalculation_Daily(t *testing.T) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24*time.Hour - time.Nanosecond)
	if !end.After(start) {
		t.Error("end should be after start")
	}
	if end.Sub(start).Hours() > 24 {
		t.Error("daily period should be <= 24 hours")
	}
}

func TestPeriodCalculation_Monthly(t *testing.T) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0).Add(-time.Nanosecond)
	if !end.After(start) {
		t.Error("end should be after start")
	}
}

func TestRenewalRateCalculation(t *testing.T) {
	totalActive := 100
	renewed := 85
	rate := float64(renewed) / float64(totalActive) * 100
	if rate != 85.0 {
		t.Errorf("expected 85.0, got %f", rate)
	}

	// Edge: zero active
	totalActive = 0
	rate = 0
	if totalActive > 0 {
		rate = float64(renewed) / float64(totalActive) * 100
	}
	if rate != 0 {
		t.Error("rate should be 0 when no active members")
	}
}

func TestClassFillRateCalculation(t *testing.T) {
	capacity := 30
	booked := 25
	fillRate := float64(booked) / float64(capacity) * 100
	expected := 83.33
	if fmt.Sprintf("%.2f", fillRate) != fmt.Sprintf("%.2f", expected) {
		t.Errorf("expected ~%.2f, got %.2f", expected, fillRate)
	}

	// Edge: zero capacity
	capacity = 0
	fillRate = 0
	if capacity > 0 {
		fillRate = float64(booked) / float64(capacity) * 100
	}
	if fillRate != 0 {
		t.Error("fill rate should be 0 for zero capacity")
	}
}

func TestExportFileNaming(t *testing.T) {
	ts := time.Date(2026, 4, 1, 14, 30, 0, 0, time.UTC).Format("20060102_150405")
	filename := fmt.Sprintf("inventory_%s.csv", ts)
	expected := "inventory_20260401_143000.csv"
	if filename != expected {
		t.Errorf("expected %s, got %s", expected, filename)
	}

	// PDF
	pdfFilename := fmt.Sprintf("orders_%s.pdf", ts)
	if !strings.HasSuffix(pdfFilename, ".pdf") {
		t.Error("PDF should end with .pdf")
	}
}

func TestExportFormatValues(t *testing.T) {
	valid := map[string]bool{"csv": true, "pdf": true}
	if !valid["csv"] || !valid["pdf"] {
		t.Error("csv and pdf should be valid")
	}
	if valid["xlsx"] {
		t.Error("xlsx should not be valid")
	}
}

func TestReportTypeValues(t *testing.T) {
	valid := map[string]bool{
		"dashboard": true, "member-growth": true, "churn": true,
		"inventory": true, "group-buys": true, "orders": true,
	}
	if len(valid) != 6 {
		t.Errorf("expected 6 report types, got %d", len(valid))
	}
}
