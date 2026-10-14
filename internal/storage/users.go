package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syragen-apibased/internal/models"
)

var (
	ErrCannotDeleteAdmin = errors.New("cannot remove administrator authorization")
	ErrUserNotFound      = errors.New("user not found")
)

// UserSettings contains generation options selected by the user.
type UserSettings struct {
	ImageModel       string `json:"image_model"`
	ImageResolution  string `json:"image_resolution"`
	ImageAspectRatio string `json:"image_aspect_ratio"`
	ImageQuality     string `json:"image_quality,omitempty"`
	VideoResolution  string `json:"video_resolution"`
	VideoDuration    int    `json:"video_duration"`
	VideoAspectRatio string `json:"video_aspect_ratio"`
}

// DefaultUserSettings returns standard default settings normalized for Grok Image 2.0 and Grok Video 1.5.
func DefaultUserSettings() UserSettings {
	return UserSettings{
		ImageModel:       models.ModelGrokImage20,
		ImageResolution:  "2K",
		ImageAspectRatio: "1:1",
		ImageQuality:     "medium",
		VideoResolution:  "720p",
		VideoDuration:    6,
		VideoAspectRatio: "16:9",
	}
}

// UserStoreFile is the schema stored in data/users.json.
type UserStoreFile struct {
	AuthorizedUsers []int64                 `json:"authorized_users"`
	Settings        map[string]UserSettings `json:"settings"` // keyed by string user ID
}

// UserStore manages authorized users and their settings.
type UserStore struct {
	filePath string
	adminID  int64
	mu       sync.RWMutex
	data     UserStoreFile
}

// NewUserStore creates and initializes a UserStore.
func NewUserStore(filePath string, adminID int64) (*UserStore, error) {
	store := &UserStore{
		filePath: filePath,
		adminID:  adminID,
		data: UserStoreFile{
			AuthorizedUsers: []int64{adminID},
			Settings:        make(map[string]UserSettings),
		},
	}

	if err := store.load(); err != nil {
		if os.IsNotExist(err) {
			// Initialize with admin authorized
			store.data.AuthorizedUsers = []int64{adminID}
			store.data.Settings[fmt.Sprintf("%d", adminID)] = DefaultUserSettings()
			if err := store.saveUnlocked(); err != nil {
				return nil, fmt.Errorf("failed to initialize %s: %w", filePath, err)
			}
			return store, nil
		}
		return nil, fmt.Errorf("failed to load %s: %w", filePath, err)
	}

	// Ensure admin is always authorized
	adminFound := false
	for _, id := range store.data.AuthorizedUsers {
		if id == adminID {
			adminFound = true
			break
		}
	}
	if !adminFound {
		store.data.AuthorizedUsers = append(store.data.AuthorizedUsers, adminID)
		_ = store.saveUnlocked()
	}

	return store, nil
}

// IsAuthorized checks if the given Telegram user ID is authorized.
func (s *UserStore) IsAuthorized(userID int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if userID == s.adminID {
		return true
	}
	for _, id := range s.data.AuthorizedUsers {
		if id == userID {
			return true
		}
	}
	return false
}

// AddUser authorizes a new Telegram user ID.
func (s *UserStore) AddUser(userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range s.data.AuthorizedUsers {
		if id == userID {
			return nil // already authorized
		}
	}

	s.data.AuthorizedUsers = append(s.data.AuthorizedUsers, userID)
	if _, ok := s.data.Settings[fmt.Sprintf("%d", userID)]; !ok {
		s.data.Settings[fmt.Sprintf("%d", userID)] = DefaultUserSettings()
	}

	return storeAndSave(s)
}

// DelUser removes an authorized Telegram user ID (cannot delete admin).
func (s *UserStore) DelUser(userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID == s.adminID {
		return ErrCannotDeleteAdmin
	}

	found := false
	var updated []int64
	for _, id := range s.data.AuthorizedUsers {
		if id == userID {
			found = true
			continue
		}
		updated = append(updated, id)
	}

	if !found {
		return ErrUserNotFound
	}

	s.data.AuthorizedUsers = updated
	delete(s.data.Settings, fmt.Sprintf("%d", userID))

	return storeAndSave(s)
}

// ListUsers returns a sorted list of all authorized Telegram user IDs.
func (s *UserStore) ListUsers() []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]int64, len(s.data.AuthorizedUsers))
	copy(res, s.data.AuthorizedUsers)
	sort.Slice(res, func(i, j int) bool { return res[i] < res[j] })
	return res
}

// GetSettings retrieves normalized settings for the user.
func (s *UserStore) GetSettings(userID int64) UserSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%d", userID)
	settings, ok := s.data.Settings[key]
	if !ok {
		settings = DefaultUserSettings()
	}

	// Normalize settings
	settings.ImageResolution, settings.ImageAspectRatio, settings.ImageQuality = models.NormalizeImageSettings(
		settings.ImageModel, settings.ImageResolution, settings.ImageAspectRatio, settings.ImageQuality,
	)
	settings.VideoResolution, settings.VideoAspectRatio, settings.VideoDuration = models.NormalizeVideoSettings(
		models.ModelGrokVideo15, settings.VideoResolution, settings.VideoAspectRatio, settings.VideoDuration,
	)

	return settings
}

// UpdateSettings updates and normalizes user settings.
func (s *UserStore) UpdateSettings(userID int64, st UserSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st.ImageResolution, st.ImageAspectRatio, st.ImageQuality = models.NormalizeImageSettings(
		st.ImageModel, st.ImageResolution, st.ImageAspectRatio, st.ImageQuality,
	)
	st.VideoResolution, st.VideoAspectRatio, st.VideoDuration = models.NormalizeVideoSettings(
		models.ModelGrokVideo15, st.VideoResolution, st.VideoAspectRatio, st.VideoDuration,
	)

	key := fmt.Sprintf("%d", userID)
	if s.data.Settings == nil {
		s.data.Settings = make(map[string]UserSettings)
	}
	s.data.Settings[key] = st

	return storeAndSave(s)
}

func (s *UserStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	var f UserStoreFile
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	if f.Settings == nil {
		f.Settings = make(map[string]UserSettings)
	}
	s.data = f
	return nil
}

func (s *UserStore) saveUnlocked() error {
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(s.filePath, data, 0644)
}

func storeAndSave(s *UserStore) error {
	return s.saveUnlocked()
}
