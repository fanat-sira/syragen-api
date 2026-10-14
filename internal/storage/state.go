package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// GenerationMetadata stores temporary details about a generated image or video.
type GenerationMetadata struct {
	ID          string    `json:"id"`
	ChatID      int64     `json:"chat_id"`
	MessageID   int       `json:"message_id"`
	UserID      int64     `json:"user_id"`
	MediaType   string    `json:"media_type"` // "image" or "video"
	LocalPath   string    `json:"local_path"`
	Prompt      string    `json:"prompt"`
	Model       string    `json:"model"`
	SourcePaths []string  `json:"source_paths,omitempty"`
	Resolution  string    `json:"resolution,omitempty"`
	AspectRatio string    `json:"aspect_ratio,omitempty"`
	Quality     string    `json:"quality,omitempty"`
	Duration    int       `json:"duration,omitempty"`
	PeriodID    string    `json:"period_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserWorkingContext stores a user's active edit or generation context.
type UserWorkingContext struct {
	ActiveEditMediaID string    `json:"active_edit_media_id,omitempty"`
	ActiveEditPath    string    `json:"active_edit_path,omitempty"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// EphemeralStateFile represents the JSON structure in runtime/state.json.
type EphemeralStateFile struct {
	Generations map[string]GenerationMetadata `json:"generations"`  // keyed by ID and "chatID:messageID"
	UserContext map[string]UserWorkingContext `json:"user_context"` // keyed by string(userID)
}

// StateStore manages ephemeral metadata and user contexts.
type StateStore struct {
	filePath string
	mu       sync.RWMutex
	data     EphemeralStateFile
}

// NewStateStore creates and initializes a StateStore.
func NewStateStore(filePath string) (*StateStore, error) {
	store := &StateStore{
		filePath: filePath,
		data: EphemeralStateFile{
			Generations: make(map[string]GenerationMetadata),
			UserContext: make(map[string]UserWorkingContext),
		},
	}

	if err := store.load(); err != nil {
		if os.IsNotExist(err) {
			_ = store.saveUnlocked()
			return store, nil
		}
		return nil, fmt.Errorf("failed to load %s: %w", filePath, err)
	}

	return store, nil
}

// SaveGeneration records metadata for a generated item.
func (s *StateStore) SaveGeneration(meta GenerationMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data.Generations == nil {
		s.data.Generations = make(map[string]GenerationMetadata)
	}

	if meta.ID != "" {
		s.data.Generations[meta.ID] = meta
	}
	if meta.ChatID != 0 && meta.MessageID != 0 {
		msgKey := fmt.Sprintf("%d:%d", meta.ChatID, meta.MessageID)
		s.data.Generations[msgKey] = meta
	}

	return s.saveUnlocked()
}

// GetGenerationByMessage finds generation metadata associated with a Telegram message.
func (s *StateStore) GetGenerationByMessage(chatID int64, messageID int) (GenerationMetadata, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgKey := fmt.Sprintf("%d:%d", chatID, messageID)
	meta, ok := s.data.Generations[msgKey]
	return meta, ok
}

// GetGenerationByID finds generation metadata by its ID.
func (s *StateStore) GetGenerationByID(id string) (GenerationMetadata, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	meta, ok := s.data.Generations[id]
	return meta, ok
}

// SetUserEditContext sets the active image to be edited by subsequent prompts.
func (s *UserWorkingContext) IsEmpty() bool {
	return s.ActiveEditPath == "" && s.ActiveEditMediaID == ""
}

// SetUserEditContext stores an active image edit context for a user.
func (s *StateStore) SetUserEditContext(userID int64, mediaID, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data.UserContext == nil {
		s.data.UserContext = make(map[string]UserWorkingContext)
	}

	key := fmt.Sprintf("%d", userID)
	s.data.UserContext[key] = UserWorkingContext{
		ActiveEditMediaID: mediaID,
		ActiveEditPath:    path,
		UpdatedAt:         time.Now(),
	}

	return s.saveUnlocked()
}

// GetUserEditContext retrieves the active edit context for a user.
func (s *StateStore) GetUserEditContext(userID int64) (UserWorkingContext, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%d", userID)
	ctx, ok := s.data.UserContext[key]
	return ctx, ok
}

// ClearUserContext clears the active working context for a user.
func (s *StateStore) ClearUserContext(userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%d", userID)
	delete(s.data.UserContext, key)
	return s.saveUnlocked()
}

// ClearAll purges all generation metadata and active contexts (e.g. during 09:00 MSK rollover).
func (s *StateStore) ClearAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Generations = make(map[string]GenerationMetadata)
	s.data.UserContext = make(map[string]UserWorkingContext)
	return s.saveUnlocked()
}

func (s *StateStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	var f EphemeralStateFile
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	if f.Generations == nil {
		f.Generations = make(map[string]GenerationMetadata)
	}
	if f.UserContext == nil {
		f.UserContext = make(map[string]UserWorkingContext)
	}
	s.data = f
	return nil
}

func (s *StateStore) saveUnlocked() error {
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(s.filePath, data, 0600)
}
