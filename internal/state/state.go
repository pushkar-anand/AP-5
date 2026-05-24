package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store persists Gmail historyId per account across restarts.
type Store struct {
	mu   sync.Mutex
	path string
	data map[string]string
}

func New(dataDir string) (*Store, error) {
	path := filepath.Join(dataDir, "state.json")

	s := &Store{
		path: path,
		data: make(map[string]string),
	}

	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("state: read %s: %w", path, err)
	}
	if b != nil {
		if err := json.Unmarshal(b, &s.data); err != nil {
			return nil, fmt.Errorf("state: parse %s: %w", path, err)
		}
	}

	return s, nil
}

func (s *Store) GetHistoryID(email string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[email]
}

func (s *Store) SetHistoryID(email, historyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[email] = historyID
	return s.save()
}

func (s *Store) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("state: marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("state: mkdir: %w", err)
	}
	return os.WriteFile(s.path, b, 0600)
}
