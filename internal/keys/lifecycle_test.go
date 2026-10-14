package keys

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syragen-apibased/internal/openrouter"
	"testing"
	"time"
)

func TestKeyLifecycle(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Moscow")
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "spend.csv")
	runtimeDir := filepath.Join(tempDir, "runtime")
	_ = os.MkdirAll(runtimeDir, 0755)

	var createCalls int32
	var getCalls int32
	var deleteCalls int32

	// Setup mock OpenRouter server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/keys":
			atomic.AddInt32(&createCalls, 1)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(openrouter.CreateKeyResponse{
				Data: openrouter.KeyData{
					Hash:  "mock_key_hash_123",
					Name:  "app",
					Usage: 0.0,
				},
				Key: "sk-or-v1-mock-created-key",
			})

		case r.Method == http.MethodGet && r.URL.Path == "/keys/mock_key_hash_123":
			atomic.AddInt32(&getCalls, 1)
			w.WriteHeader(http.StatusOK)
			limit := 4.0
			_ = json.NewEncoder(w).Encode(openrouter.GetKeyResponse{
				Data: openrouter.KeyData{
					Hash:  "mock_key_hash_123",
					Name:  "app",
					Usage: 2.7341,
					Limit: &limit,
				},
			})

		case r.Method == http.MethodDelete && r.URL.Path == "/keys/mock_key_hash_123":
			atomic.AddInt32(&deleteCalls, 1)
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	orClient := openrouter.NewClient(server.URL, server.Client())
	spendRecorder := NewSpendRecorder(csvPath)

	km := NewManager(orClient, spendRecorder, "test_mgmt_key", 4.0, loc, runtimeDir)

	ctx := context.Background()

	// 1. First request lazily creates a key
	key1, err := km.GetOrCreateActiveKey(ctx, nil)
	if err != nil {
		t.Fatalf("failed to get active key: %v", err)
	}
	if key1 != "sk-or-v1-mock-created-key" {
		t.Errorf("expected mock key, got %s", key1)
	}
	if atomic.LoadInt32(&createCalls) != 1 {
		t.Errorf("expected exactly 1 create call, got %d", atomic.LoadInt32(&createCalls))
	}

	// 2. Second request reuses the key
	key2, err := km.GetOrCreateActiveKey(ctx, nil)
	if err != nil {
		t.Fatalf("failed to get active key on second call: %v", err)
	}
	if key2 != key1 {
		t.Errorf("expected reused key, got %s", key2)
	}
	if atomic.LoadInt32(&createCalls) != 1 {
		t.Errorf("create was called again; expected 1, got %d", atomic.LoadInt32(&createCalls))
	}

	// 3. Concurrent requests create only one key
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			k, err := km.GetOrCreateActiveKey(ctx, nil)
			if err != nil || k != "sk-or-v1-mock-created-key" {
				t.Errorf("concurrent get error: %v", err)
			}
		}()
	}
	wg.Wait()
	if atomic.LoadInt32(&createCalls) != 1 {
		t.Errorf("concurrent calls created multiple keys: %d", atomic.LoadInt32(&createCalls))
	}

	// 4. Test Spend Query
	spent, limit, hasKey, _, err := km.GetCurrentSpend(ctx)
	if err != nil || !hasKey || spent != 2.7341 || limit != 4.0 {
		t.Errorf("expected spend 2.7341, limit 4.0; got spent=%f, limit=%f, err=%v", spent, limit, err)
	}

	// 5. Test Restart in same period
	kmRestarted := NewManager(orClient, spendRecorder, "test_mgmt_key", 4.0, loc, runtimeDir)
	if err := kmRestarted.RecoverOnStartup(ctx, nil); err != nil {
		t.Fatalf("startup recovery error: %v", err)
	}
	keyRecovered, err := kmRestarted.GetOrCreateActiveKey(ctx, nil)
	if err != nil || keyRecovered != key1 {
		t.Errorf("expected recovered key %s, got %s (err=%v)", key1, keyRecovered, err)
	}
	if atomic.LoadInt32(&createCalls) != 1 {
		t.Errorf("restart caused extra key creation: %d", atomic.LoadInt32(&createCalls))
	}

	// 6. Test Limit Exhausted handling
	kmRestarted.MarkLimitExhausted()
	_, err = kmRestarted.GetOrCreateActiveKey(ctx, nil)
	if err != ErrDailyLimitExhausted {
		t.Errorf("expected ErrDailyLimitExhausted, got %v", err)
	}
	// Verify no replacement key was created
	if atomic.LoadInt32(&createCalls) != 1 {
		t.Errorf("unexpected key creation when limit exhausted: %d", atomic.LoadInt32(&createCalls))
	}

	// 7. Test 09:00 Finalization
	cleaned := false
	cleanupCb := func() {
		cleaned = true
	}
	kmRestarted.FinalizePeriod(ctx, cleanupCb)
	if !cleaned {
		t.Error("cleanup callback was not called during finalization")
	}
	if atomic.LoadInt32(&deleteCalls) != 1 {
		t.Errorf("expected 1 delete call, got %d", atomic.LoadInt32(&deleteCalls))
	}

	// Verify spend was recorded in CSV
	currentPeriod := GetPeriod(time.Now(), loc)
	savedSpend, ok, _ := spendRecorder.GetSpend(currentPeriod.ID)
	if !ok || savedSpend != 2.7341 {
		t.Errorf("expected recorded spend 2.7341 for period %s, got %f (ok=%v)", currentPeriod.ID, savedSpend, ok)
	}

	// 8. Test Startup Recovery of Expired Key from Previous Period
	expiredState := ActiveKeyState{
		Hash:      "mock_key_hash_123",
		APIKey:    "sk-or-v1-old-key",
		PeriodID:  "2020-01-01",
		ExpiresAt: time.Date(2020, 1, 2, 9, 0, 0, 0, loc),
	}
	expiredData, _ := json.Marshal(expiredState)
	_ = os.WriteFile(filepath.Join(runtimeDir, "active_key.json"), expiredData, 0600)

	cleanedExpired := false
	kmExpired := NewManager(orClient, spendRecorder, "test_mgmt_key", 4.0, loc, runtimeDir)
	if err := kmExpired.RecoverOnStartup(ctx, func() { cleanedExpired = true }); err != nil {
		t.Fatalf("error recovering expired key: %v", err)
	}

	if !cleanedExpired {
		t.Error("expected cleanup of expired period during startup")
	}

	// Verify old active_key.json was removed
	if _, err := os.Stat(filepath.Join(runtimeDir, "active_key.json")); !os.IsNotExist(err) {
		t.Error("expected active_key.json to be deleted after expired period recovery")
	}
}
