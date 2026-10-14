package keys

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syragen-apibased/internal/storage"
)

// SpendRecorder manages the permanent spend.csv record.
type SpendRecorder struct {
	filePath string
	mu       sync.Mutex
}

// NewSpendRecorder creates a new SpendRecorder.
func NewSpendRecorder(filePath string) *SpendRecorder {
	return &SpendRecorder{
		filePath: filePath,
	}
}

// RecordSpend records or updates the total spent amount for a specific period date.
func (r *SpendRecorder) RecordSpend(date string, spentUSD float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	records, err := r.loadRecordsUnlocked()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing spend records: %w", err)
	}

	if records == nil {
		records = make(map[string]float64)
	}

	// Always record the latest known spend
	records[date] = spentUSD

	return r.saveRecordsUnlocked(records)
}

// GetSpend retrieves the recorded spend for a given period date.
func (r *SpendRecorder) GetSpend(date string) (float64, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	records, err := r.loadRecordsUnlocked()
	if err != nil {
		if os.IsNotExist(err) {
			return 0, false, nil
		}
		return 0, false, err
	}

	val, ok := records[date]
	return val, ok, nil
}

func (r *SpendRecorder) loadRecordsUnlocked() (map[string]float64, error) {
	file, err := os.Open(r.filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse csv: %w", err)
	}

	records := make(map[string]float64)
	for i, row := range rows {
		if i == 0 {
			// Skip header row
			continue
		}
		if len(row) < 2 {
			continue
		}
		dateStr := strings.TrimSpace(row[0])
		spentStr := strings.TrimSpace(row[1])
		if dateStr == "" {
			continue
		}
		spentVal, err := strconv.ParseFloat(spentStr, 64)
		if err != nil {
			continue
		}
		records[dateStr] = spentVal
	}

	return records, nil
}

func (r *SpendRecorder) saveRecordsUnlocked(records map[string]float64) error {
	var dates []string
	for d := range records {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	var sb strings.Builder
	sb.WriteString("date,spent_usd\n")
	for _, d := range dates {
		sb.WriteString(fmt.Sprintf("%s,%.4f\n", d, records[d]))
	}

	if err := os.MkdirAll(filepath.Dir(r.filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for spend.csv: %w", err)
	}

	return storage.WriteFileAtomic(r.filePath, []byte(sb.String()), 0644)
}
