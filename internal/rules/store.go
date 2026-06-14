package rules

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	ActionImportTransaction = "import_transaction"
	ActionLogOnly           = "log_only"
)

// Rule is a user-taught handling instruction for a specific email category.
type Rule struct {
	Category         string    `json:"category"`          // LLM category string (e.g. "bank_account_transaction")
	ExtractionPrompt string    `json:"extraction_prompt"` // user-written LLM extraction prompt
	Action           string    `json:"action"`            // ActionImportTransaction or ActionLogOnly
	CreatedAt        time.Time `json:"created_at"`
}

// Store persists learned rules to disk as JSON.
type Store struct {
	mu   sync.RWMutex
	path string
	data []Rule
}

// NewStore loads (or creates) the rule store at dataDir/learned_rules.json.
func NewStore(dataDir string) (*Store, error) {
	path := filepath.Join(dataDir, "learned_rules.json")

	s := &Store{path: path}

	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("rules store: read %s: %w", path, err)
	}
	if b != nil {
		if err := json.Unmarshal(b, &s.data); err != nil {
			return nil, fmt.Errorf("rules store: parse %s: %w", path, err)
		}
	}

	return s, nil
}

// Save adds a new rule or replaces an existing one with the same category.
func (s *Store) Save(rule Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now()
	}

	for i, r := range s.data {
		if r.Category == rule.Category {
			s.data[i] = rule
			return s.save()
		}
	}

	s.data = append(s.data, rule)
	return s.save()
}

// List returns all stored rules.
func (s *Store) List() []Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Rule, len(s.data))
	copy(out, s.data)
	return out
}

// Get returns the rule for the given category.
func (s *Store) Get(category string) (Rule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.data {
		if r.Category == category {
			return r, true
		}
	}
	return Rule{}, false
}

// Delete removes the rule for the given category.
func (s *Store) Delete(category string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated := s.data[:0]
	for _, r := range s.data {
		if r.Category != category {
			updated = append(updated, r)
		}
	}
	s.data = updated
	return s.save()
}

func (s *Store) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("rules store: marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("rules store: mkdir: %w", err)
	}
	return os.WriteFile(s.path, b, 0600)
}
