package keys

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"syragen-apibased/internal/openrouter"
	"syragen-apibased/internal/storage"
	"time"
)

var (
	ErrDailyLimitExhausted = errors.New("the daily generation budget ($4 USD) has been exhausted. Generations will be available again at 09:00 MSK")
)

// ActiveKeyState represents the persisted state of the active ephemeral API key.
type ActiveKeyState struct {
	Hash           string    `json:"hash"`
	APIKey         string    `json:"api_key"`
	PeriodID       string    `json:"period_id"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	LimitExhausted bool      `json:"limit_exhausted"`
}

// Manager handles the lifecycle of OpenRouter ephemeral generation keys.
type Manager struct {
	client        *openrouter.Client
	spendRecorder *SpendRecorder
	managementKey string
	spendLimit    float64
	timezone      *time.Location
	runtimeDir    string
	activeKeyPath string

	mu        sync.Mutex
	activeKey *ActiveKeyState
}

// NewManager creates a new key lifecycle Manager.
func NewManager(client *openrouter.Client, spendRecorder *SpendRecorder, managementKey string, spendLimit float64, loc *time.Location, runtimeDir string) *Manager {
	if loc == nil {
		loc = time.UTC
	}
	if spendLimit <= 0 {
		spendLimit = 4.0
	}
	return &Manager{
		client:        client,
		spendRecorder: spendRecorder,
		managementKey: managementKey,
		spendLimit:    spendLimit,
		timezone:      loc,
		runtimeDir:    runtimeDir,
		activeKeyPath: filepath.Join(runtimeDir, "active_key.json"),
	}
}

// RecoverOnStartup inspects runtime state and finalizes any expired key before serving requests.
func (m *Manager) RecoverOnStartup(ctx context.Context, onCleanup func()) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.loadActiveKeyFileUnlocked()
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[lifecycle] startup recovery: no existing active key file found")
			return nil
		}
		log.Printf("[lifecycle] startup recovery: warning loading active key: %v", err)
	}

	if state == nil {
		return nil
	}

	now := time.Now()
	currentPeriod := GetPeriod(now, m.timezone)

	// Check if the loaded key belongs to an expired period or its expiration time has passed
	if state.PeriodID != currentPeriod.ID || now.After(state.ExpiresAt) || now.Equal(state.ExpiresAt) {
		log.Printf("[lifecycle] startup recovery: detected expired key for period %s (current is %s). Finalizing...", state.PeriodID, currentPeriod.ID)
		m.finalizeKeyUnlocked(ctx, state, onCleanup)
		m.activeKey = nil
		_ = os.Remove(m.activeKeyPath)
		return nil
	}

	// Key is still valid for the current period
	log.Printf("[lifecycle] startup recovery: active key for current period %s is valid until %s. Reusing.", state.PeriodID, state.ExpiresAt.Format(time.RFC3339))
	m.activeKey = state
	return nil
}

// GetOrCreateActiveKey returns the active generation API key for the current period, creating one lazily if needed.
func (m *Manager) GetOrCreateActiveKey(ctx context.Context, onCleanup func()) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	currentPeriod := GetPeriod(now, m.timezone)

	// If we have an active key, verify if it's still for current period and not expired
	if m.activeKey != nil {
		if m.activeKey.PeriodID == currentPeriod.ID && now.Before(m.activeKey.ExpiresAt) {
			if m.activeKey.LimitExhausted {
				return "", ErrDailyLimitExhausted
			}
			return m.activeKey.APIKey, nil
		}

		// Key has expired or belongs to an old period: finalize it now
		log.Printf("[lifecycle] active key expired (period %s, current %s). Finalizing...", m.activeKey.PeriodID, currentPeriod.ID)
		m.finalizeKeyUnlocked(ctx, m.activeKey, onCleanup)
		m.activeKey = nil
		_ = os.Remove(m.activeKeyPath)
	}

	// No active key for current period: lazily create one
	log.Printf("[lifecycle] creating new ephemeral OpenRouter key for period %s (expires at %s MSK)", currentPeriod.ID, currentPeriod.EndTime.Format(time.RFC3339))

	resp, err := m.client.CreateKey(ctx, m.managementKey, "app", m.spendLimit, currentPeriod.EndTime)
	if err != nil {
		return "", fmt.Errorf("failed to create ephemeral OpenRouter key: %w", err)
	}

	state := &ActiveKeyState{
		Hash:           resp.Data.Hash,
		APIKey:         resp.Key,
		PeriodID:       currentPeriod.ID,
		CreatedAt:      time.Now(),
		ExpiresAt:      currentPeriod.EndTime,
		LimitExhausted: false,
	}

	if err := m.saveActiveKeyFileUnlocked(state); err != nil {
		log.Printf("[lifecycle] warning: failed to persist active key to %s: %v", m.activeKeyPath, err)
	}

	m.activeKey = state
	log.Printf("[lifecycle] successfully created and activated key for period %s", currentPeriod.ID)
	return m.activeKey.APIKey, nil
}

// MarkLimitExhausted flags the current active key as exhausted so no new generation calls are made until next period.
func (m *Manager) MarkLimitExhausted() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeKey != nil {
		m.activeKey.LimitExhausted = true
		_ = m.saveActiveKeyFileUnlocked(m.activeKey)
		log.Printf("[lifecycle] marked active key as exhausted for period %s", m.activeKey.PeriodID)
	}
}

// GetCurrentSpend queries OpenRouter for the spend of the active key in the current period.
func (m *Manager) GetCurrentSpend(ctx context.Context) (spent float64, limit float64, hasKey bool, periodID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentPeriod := GetPeriod(time.Now(), m.timezone)
	limit = m.spendLimit
	periodID = currentPeriod.ID

	if m.activeKey == nil || m.activeKey.PeriodID != currentPeriod.ID {
		return 0.0, limit, false, periodID, nil
	}

	// Query spend from OpenRouter Management API
	keyResp, err := m.client.GetKey(ctx, m.managementKey, m.activeKey.Hash)
	if err != nil {
		return 0.0, limit, true, periodID, fmt.Errorf("failed to query key status: %w", err)
	}

	if keyResp.Data.Limit != nil {
		limit = *keyResp.Data.Limit
	}

	return keyResp.Data.Usage, limit, true, periodID, nil
}

// FinalizePeriod finalizes the active period at the 09:00 boundary.
func (m *Manager) FinalizePeriod(ctx context.Context, onCleanup func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("[lifecycle] 09:00 MSK period rollover triggered")
	if m.activeKey != nil {
		m.finalizeKeyUnlocked(ctx, m.activeKey, onCleanup)
		m.activeKey = nil
		_ = os.Remove(m.activeKeyPath)
	} else if onCleanup != nil {
		onCleanup()
	}
}

func (m *Manager) finalizeKeyUnlocked(ctx context.Context, state *ActiveKeyState, onCleanup func()) {
	if state == nil || state.Hash == "" {
		if onCleanup != nil {
			onCleanup()
		}
		return
	}

	// 1. Query final spend
	var spent float64
	keyResp, err := m.client.GetKey(ctx, m.managementKey, state.Hash)
	if err == nil && keyResp != nil {
		spent = keyResp.Data.Usage
		log.Printf("[lifecycle] finalized spend for period %s: $%.4f", state.PeriodID, spent)
	} else {
		log.Printf("[lifecycle] warning: failed to query final spend for key %s (period %s): %v", state.Hash, state.PeriodID, err)
		// Check if spend already recorded in CSV
		if existing, ok, _ := m.spendRecorder.GetSpend(state.PeriodID); ok {
			spent = existing
		}
	}

	// 2. Record to spend.csv
	if err := m.spendRecorder.RecordSpend(state.PeriodID, spent); err != nil {
		log.Printf("[lifecycle] error recording spend to CSV for period %s: %v", state.PeriodID, err)
	}

	// 3. Delete / revoke remote key
	if err := m.client.DeleteKey(ctx, m.managementKey, state.Hash); err != nil {
		if apiErr, ok := err.(*openrouter.APIError); ok && apiErr.StatusCode == http.StatusNotFound {
			// Already deleted
		} else {
			log.Printf("[lifecycle] warning: failed to delete key %s from OpenRouter: %v", state.Hash, err)
		}
	} else {
		log.Printf("[lifecycle] revoked remote key %s for period %s", state.Hash, state.PeriodID)
	}

	// 4. Perform media and ephemeral state cleanup
	if onCleanup != nil {
		onCleanup()
	}
}

func (m *Manager) loadActiveKeyFileUnlocked() (*ActiveKeyState, error) {
	data, err := os.ReadFile(m.activeKeyPath)
	if err != nil {
		return nil, err
	}
	var state ActiveKeyState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse active key state: %w", err)
	}
	return &state, nil
}

func (m *Manager) saveActiveKeyFileUnlocked(state *ActiveKeyState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return storage.WriteFileAtomic(m.activeKeyPath, data, 0600)
}
