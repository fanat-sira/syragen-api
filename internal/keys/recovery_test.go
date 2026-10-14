package keys

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syragen-apibased/internal/openrouter"
	"testing"
	"time"
)

func TestFailedRevocationSurvivesRestartAndBlocksReplacement(t *testing.T) {
	var failDelete atomic.Bool
	failDelete.Store(true)
	var created atomic.Int32
	var deleted atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(openrouter.GetKeyResponse{Data: openrouter.KeyData{Usage: 1.25}})
		case http.MethodDelete:
			deleted.Add(1)
			if failDelete.Load() {
				w.WriteHeader(403)
				return
			}
			w.WriteHeader(204)
		case http.MethodPost:
			created.Add(1)
			_ = json.NewEncoder(w).Encode(openrouter.CreateKeyResponse{Data: openrouter.KeyData{Hash: "replacement-hash"}, Key: "test-replacement-key"})
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	recorder := NewSpendRecorder(filepath.Join(dir, "spend.csv"))
	client := openrouter.NewClient(server.URL, server.Client())
	makeManager := func() *Manager { return NewManager(client, recorder, "test-management", 7.5, time.UTC, dir) }
	manager := makeManager()
	// A pending finalization must resume even if the original key was in this period.
	period := GetPeriod(time.Now(), time.UTC)
	state := &ActiveKeyState{Hash: "old-hash", APIKey: "test-old-key", PeriodID: period.ID, ExpiresAt: period.EndTime}
	if err := manager.saveActiveKeyFileUnlocked(state); err != nil {
		t.Fatal(err)
	}
	if err := manager.RecoverOnStartup(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	cleaned := 0
	cleanup := func() { cleaned++ }
	if err := manager.FinalizePeriod(context.Background(), cleanup); err == nil {
		t.Fatal("expected failed revoke")
	}
	raw, err := os.ReadFile(manager.activeKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	var persisted ActiveKeyState
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatal(err)
	}
	if !persisted.FinalizationPending || persisted.FinalSpend == nil || *persisted.FinalSpend != 1.25 {
		t.Fatalf("recovery information missing: %+v", persisted)
	}
	if cleaned != 0 {
		t.Fatal("cleaned state before revocation")
	}
	restarted := makeManager()
	if err := restarted.RecoverOnStartup(context.Background(), cleanup); err == nil {
		t.Fatal("expected startup retry failure")
	}
	if _, err := restarted.GetOrCreateActiveKey(context.Background(), cleanup); err == nil {
		t.Fatal("replacement permitted before revoke")
	}
	if created.Load() != 0 {
		t.Fatal("created replacement while old key remained")
	}
	failDelete.Store(false)
	if err := restarted.RetryFinalization(context.Background(), cleanup); err != nil {
		t.Fatal(err)
	}
	if cleaned != 1 || restarted.activeKey != nil {
		t.Fatal("finalization did not clean up")
	}
	if _, err := os.Stat(restarted.activeKeyPath); !os.IsNotExist(err) {
		t.Fatalf("key file not removed: %v", err)
	}
	if deleted.Load() != 4 {
		t.Fatalf("expected revoke attempts at rollover, restart, request, background; got %d", deleted.Load())
	}
	spend, ok, err := recorder.GetSpend(period.ID)
	if err != nil || !ok || spend != 1.25 {
		t.Fatalf("spend %v, %v, %v", spend, ok, err)
	}
	if _, err := restarted.GetOrCreateActiveKey(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if created.Load() != 1 {
		t.Fatal("new key was not allowed after confirmed revocation")
	}
}

func TestBrokenStateNeverCreatesReplacement(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "active_key.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	manager := NewManager(openrouter.NewClient(server.URL, server.Client()), NewSpendRecorder(filepath.Join(dir, "spend.csv")), "test", 4, time.UTC, dir)
	if err := manager.RecoverOnStartup(context.Background(), nil); err == nil {
		t.Fatal("accepted broken state")
	}
	if _, err := manager.GetOrCreateActiveKey(context.Background(), nil); err == nil {
		t.Fatal("ignored broken persisted state")
	}
	if calls.Load() != 0 {
		t.Fatal("contacted provider despite broken local state")
	}
}

func TestConfiguredPeriodAndLimitMessage(t *testing.T) {
	loc := time.FixedZone("UTC-05", -5*60*60)
	manager := NewManager(nil, nil, "", 7.5, loc, t.TempDir())
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	if got := manager.CurrentPeriod(now).ID; got != "2026-10-13" {
		t.Fatalf("wrong configured period: %s", got)
	}
	if msg := manager.LimitMessage(); !strings.Contains(msg, "$7.5 USD") || !strings.Contains(msg, "09:00 UTC-05") {
		t.Fatal(msg)
	}
}

func TestAccountingFailureRetainsKeyUntilRetry(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	var deletes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deletes.Add(1)
			w.WriteHeader(204)
			return
		}
		_ = json.NewEncoder(w).Encode(openrouter.GetKeyResponse{Data: openrouter.KeyData{Usage: 2.5}})
	}))
	defer server.Close()
	manager := NewManager(openrouter.NewClient(server.URL, server.Client()), NewSpendRecorder(filepath.Join(blocked, "spend.csv")), "test", 4, time.UTC, dir)
	period := manager.CurrentPeriod(time.Now())
	manager.activeKey = &ActiveKeyState{Hash: "test-hash", APIKey: "test-key", PeriodID: period.ID, ExpiresAt: period.EndTime}
	cleaned := false
	if err := manager.FinalizePeriod(context.Background(), func() { cleaned = true }); err == nil {
		t.Fatal("expected accounting failure")
	}
	if cleaned || deletes.Load() != 0 {
		t.Fatal("revoked or cleaned before recording spend")
	}
	if _, err := os.Stat(manager.activeKeyPath); err != nil {
		t.Fatal("lost key state", err)
	}
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	if err := manager.RetryFinalization(context.Background(), func() { cleaned = true }); err != nil {
		t.Fatal(err)
	}
	if !cleaned || deletes.Load() != 1 {
		t.Fatal("retry did not finish")
	}
}

func TestAlreadyDeletedKeyCompletesRecoveryWithoutLosingKnownSpend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	defer server.Close()
	dir := t.TempDir()
	recorder := NewSpendRecorder(filepath.Join(dir, "spend.csv"))
	manager := NewManager(openrouter.NewClient(server.URL, server.Client()), recorder, "test", 4, time.UTC, dir)
	spend := 1.75
	state := &ActiveKeyState{Hash: "deleted-hash", APIKey: "test-key", PeriodID: "2020-01-01", ExpiresAt: time.Date(2020, 1, 2, 9, 0, 0, 0, time.UTC), FinalizationPending: true, FinalSpend: &spend}
	if err := manager.saveActiveKeyFileUnlocked(state); err != nil {
		t.Fatal(err)
	}
	cleaned := false
	if err := manager.RecoverOnStartup(context.Background(), func() { cleaned = true }); err != nil {
		t.Fatal(err)
	}
	value, exists, err := recorder.GetSpend(state.PeriodID)
	if err != nil || !exists || value != spend || !cleaned {
		t.Fatalf("lost recovery spend: %v, %v, %v, cleaned=%v", value, exists, err, cleaned)
	}
}

func TestNewKeyIsNotServedWhenPersistenceFails(t *testing.T) {
	dir := t.TempDir()
	var deleted atomic.Int32
	manager := (*Manager)(nil)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// A storage failure appearing after the remote create succeeded.
			if err := os.Mkdir(manager.activeKeyPath, 0700); err != nil {
				t.Error(err)
			}
			_ = json.NewEncoder(w).Encode(openrouter.CreateKeyResponse{Data: openrouter.KeyData{Hash: "unpersisted-hash"}, Key: "test-unpersisted-key"})
			return
		}
		if r.Method == http.MethodDelete {
			deleted.Add(1)
			w.WriteHeader(204)
			return
		}
		t.Error("unexpected provider request", r.Method)
	}))
	defer server.Close()
	manager = NewManager(openrouter.NewClient(server.URL, server.Client()), NewSpendRecorder(filepath.Join(dir, "spend.csv")), "test", 4, time.UTC, dir)
	key, err := manager.GetOrCreateActiveKey(context.Background(), nil)
	if err == nil || key != "" || deleted.Load() != 1 {
		t.Fatalf("served an unpersisted key or failed compensation: key=%q, err=%v, deleted=%d", key, err, deleted.Load())
	}
}

func TestDelayedRolloverDoesNotRevokeNewPeriodKey(t *testing.T) {
	dir := t.TempDir()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	manager := NewManager(openrouter.NewClient(server.URL, server.Client()), NewSpendRecorder(filepath.Join(dir, "spend.csv")), "test", 4, time.UTC, dir)
	period := manager.CurrentPeriod(time.Now())
	manager.activeKey = &ActiveKeyState{Hash: "new-period-hash", APIKey: "test-key", PeriodID: period.ID, ExpiresAt: period.EndTime}
	if err := manager.saveActiveKeyFileUnlocked(manager.activeKey); err != nil {
		t.Fatal(err)
	}
	cleaned := false
	if err := manager.FinalizeExpiredPeriod(context.Background(), func() { cleaned = true }); err != nil {
		t.Fatal(err)
	}
	if cleaned || calls.Load() != 0 || manager.activeKey == nil {
		t.Fatal("delayed scheduler touched the new period's key")
	}
	if err := manager.RetryFinalization(context.Background(), func() { cleaned = true }); err != nil {
		t.Fatal(err)
	}
	if cleaned || calls.Load() != 0 {
		t.Fatal("background retry touched a valid key")
	}
}
