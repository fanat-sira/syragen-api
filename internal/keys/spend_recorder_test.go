package keys

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpendRecorder(t *testing.T) {
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "spend.csv")

	recorder := NewSpendRecorder(csvPath)

	// 1. Initial write
	if err := recorder.RecordSpend("2026-08-25", 2.7341); err != nil {
		t.Fatalf("failed to record spend: %v", err)
	}

	val, ok, err := recorder.GetSpend("2026-08-25")
	if err != nil || !ok || val != 2.7341 {
		t.Errorf("expected 2.7341, got %f (ok=%v, err=%v)", val, ok, err)
	}

	// 2. Update same date (deduplication)
	if err := recorder.RecordSpend("2026-08-25", 3.1250); err != nil {
		t.Fatalf("failed to update spend: %v", err)
	}

	val, ok, err = recorder.GetSpend("2026-08-25")
	if err != nil || !ok || val != 3.1250 {
		t.Errorf("expected updated 3.1250, got %f", val)
	}

	// 3. Add second date
	if err := recorder.RecordSpend("2026-08-24", 1.5000); err != nil {
		t.Fatalf("failed to record second date: %v", err)
	}

	// Read raw file content to verify formatting and sorting
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("failed to read csv file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines in csv (header + 2 rows), got %d: %q", len(lines), lines)
	}

	if lines[0] != "date,spent_usd" {
		t.Errorf("expected header 'date,spent_usd', got %q", lines[0])
	}
	if lines[1] != "2026-08-24,1.5000" {
		t.Errorf("expected sorted first line '2026-08-24,1.5000', got %q", lines[1])
	}
	if lines[2] != "2026-08-25,3.1250" {
		t.Errorf("expected sorted second line '2026-08-25,3.1250', got %q", lines[2])
	}
}
