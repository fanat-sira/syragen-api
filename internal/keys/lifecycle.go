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
	ErrDailyLimitExhausted = errors.New("daily generation budget exhausted")
)

// ActiveKeyState represents the persisted state of the active ephemeral API key.
type ActiveKeyState struct {
	Hash                string    `json:"hash"`
	APIKey              string    `json:"api_key"`
	PeriodID            string    `json:"period_id"`
	CreatedAt           time.Time `json:"created_at"`
	ExpiresAt           time.Time `json:"expires_at"`
	LimitExhausted      bool      `json:"limit_exhausted"`
	FinalizationPending bool      `json:"finalization_pending,omitempty"`
	FinalSpend          *float64  `json:"final_spend,omitempty"`
	Revoked             bool      `json:"revoked,omitempty"`
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
			return nil
		}
		return fmt.Errorf("failed to load active key state: %w", err)
	}
	m.activeKey = state
	now := time.Now()
	if state.FinalizationPending || state.PeriodID != m.CurrentPeriod(now).ID || !now.Before(state.ExpiresAt) {
		return m.finalizeActiveKeyUnlocked(ctx, onCleanup)
	}
	return nil
}

// GetOrCreateActiveKey returns the active generation API key for the current period, creating one lazily if needed.
func (m *Manager) GetOrCreateActiveKey(ctx context.Context, onCleanup func()) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// A failed startup load must never silently permit a replacement key.
	if m.activeKey == nil {
		state, err := m.loadActiveKeyFileUnlocked()
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to load active key state: %w", err)
		}
		m.activeKey = state
	}
	now := time.Now()
	currentPeriod := m.CurrentPeriod(now)

	// If we have an active key, verify if it's still for current period and not expired
	if m.activeKey != nil {
		if !m.activeKey.FinalizationPending && m.activeKey.PeriodID == currentPeriod.ID && now.Before(m.activeKey.ExpiresAt) {
			if m.activeKey.LimitExhausted {
				return "", ErrDailyLimitExhausted
			}
			return m.activeKey.APIKey, nil
		}

		// Key has expired or belongs to an old period: finalize it now
		log.Printf("[lifecycle] active key expired (period %s, current %s). Finalizing...", m.activeKey.PeriodID, currentPeriod.ID)
		if err := m.finalizeActiveKeyUnlocked(ctx, onCleanup); err != nil {
			return "", err
		}
	}

	// No active key for current period: lazily create one
	log.Printf("[lifecycle] creating new ephemeral OpenRouter key for period %s (expires at %s)", currentPeriod.ID, currentPeriod.EndTime.Format(time.RFC3339))

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

	m.activeKey = state
	if err := m.saveActiveKeyFileUnlocked(state); err != nil {
		state.FinalizationPending = true
		// Do not serve a credential we could not persist for restart recovery.
		revokeErr := m.client.DeleteKey(ctx, m.managementKey, state.Hash)
		if revokeErr == nil {
			m.activeKey = nil
		}
		return "", fmt.Errorf("failed to persist new key: %w", errors.Join(err, revokeErr))
	}
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

// CurrentPeriod uses the same configured timezone as key accounting.
func (m *Manager) CurrentPeriod(now time.Time) Period { return GetPeriod(now, m.timezone) }
func (m *Manager) TimezoneName() string               { return m.timezone.String() }
func (m *Manager) LimitMessage() string {
	return fmt.Sprintf("❌ The daily generation budget ($%g USD) has been exhausted. Generations will be available again at 09:00 %s.", m.spendLimit, m.timezone.String())
}

// FinalizePeriod retains the active key until accounting and remote revocation succeed.
func (m *Manager) FinalizePeriod(ctx context.Context, onCleanup func()) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeKey == nil {
		if onCleanup != nil {
			onCleanup()
		}
		return nil
	}
	return m.finalizeActiveKeyUnlocked(ctx, onCleanup)
}

// FinalizeExpiredPeriod is used at the scheduled boundary. A request may
// already have created today's key before the scheduler obtains the lock.
func (m *Manager) FinalizeExpiredPeriod(ctx context.Context, onCleanup func()) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeKey == nil {
		if onCleanup != nil {
			onCleanup()
		}
		return nil
	}
	return m.finalizeDueKeyUnlocked(ctx, onCleanup)
}

// RetryFinalization retries pending/expired keys without touching a current valid key.
func (m *Manager) RetryFinalization(ctx context.Context, onCleanup func()) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.finalizeDueKeyUnlocked(ctx, onCleanup)
}

func (m *Manager) finalizeDueKeyUnlocked(ctx context.Context, onCleanup func()) error {
	if m.activeKey == nil {
		return nil
	}
	now := time.Now()
	if !m.activeKey.FinalizationPending && m.activeKey.PeriodID == m.CurrentPeriod(now).ID && now.Before(m.activeKey.ExpiresAt) {
		return nil
	}
	return m.finalizeActiveKeyUnlocked(ctx, onCleanup)
}

// RunFinalizationRetries stops with ctx and bounds each retry separately.
func (m *Manager) RunFinalizationRetries(ctx context.Context, onCleanup func()) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			retryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			if err := m.RetryFinalization(retryCtx, onCleanup); err != nil {
				log.Printf("[lifecycle] finalization pending: %v", err)
			}
			cancel()
		}
	}
}

func (m *Manager) finalizeActiveKeyUnlocked(ctx context.Context, onCleanup func()) error {
	state := m.activeKey
	if state == nil || state.Hash == "" {
		return errors.New("cannot finalize missing key metadata")
	}
	state.FinalizationPending = true
	// Persist the intent before any remote operation, including for same-period retries.
	if err := m.saveActiveKeyFileUnlocked(state); err != nil {
		return fmt.Errorf("failed to persist finalization: %w", err)
	}
	if !state.Revoked {
		keyResp, err := m.client.GetKey(ctx, m.managementKey, state.Hash)
		if err != nil {
			var apiErr *openrouter.APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
				return fmt.Errorf("failed to query final spend: %w", err)
			}
			// Already absent upstream. Never invent a zero spend record.
			state.Revoked = true
			if spend, exists, readErr := m.spendRecorder.GetSpend(state.PeriodID); readErr != nil {
				return readErr
			} else if exists && state.FinalSpend == nil {
				state.FinalSpend = &spend
			}
			log.Printf("[lifecycle] remote key already absent for period %s", state.PeriodID)
		} else {
			spend := keyResp.Data.Usage
			state.FinalSpend = &spend
		}
		if err := m.saveActiveKeyFileUnlocked(state); err != nil {
			return err
		}
	}
	if state.FinalSpend != nil {
		if err := m.spendRecorder.RecordSpend(state.PeriodID, *state.FinalSpend); err != nil {
			return fmt.Errorf("failed to record final spend: %w", err)
		}
	}
	if !state.Revoked {
		if err := m.client.DeleteKey(ctx, m.managementKey, state.Hash); err != nil {
			return fmt.Errorf("failed to revoke key: %w", err)
		}
		state.Revoked = true
		if err := m.saveActiveKeyFileUnlocked(state); err != nil {
			return err
		}
	}
	if err := os.Remove(m.activeKeyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove finalized key state: %w", err)
	}
	m.activeKey = nil
	if onCleanup != nil {
		onCleanup()
	}
	return nil
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
