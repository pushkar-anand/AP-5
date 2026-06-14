package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Item is an email that arrived but had no registered handler — queued for user review.
type Item struct {
	ID            string    `json:"id"`      // Gmail message ID
	Account       string    `json:"account"` // email address that was polled
	Subject       string    `json:"subject"`
	Body          string    `json:"body"`           // full body, needed for reprocessing
	SuggestedType string    `json:"suggested_type"` // LLM's classification
	QueuedAt      time.Time `json:"queued_at"`
}

// Queue persists review items to disk as JSON.
type Queue struct {
	mu   sync.RWMutex
	path string
	data []Item
}

// NewQueue loads (or creates) the review queue at dataDir/review_queue.json.
func NewQueue(dataDir string) (*Queue, error) {
	path := filepath.Join(dataDir, "review_queue.json")

	q := &Queue{path: path}

	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("review queue: read %s: %w", path, err)
	}
	if b != nil {
		if err := json.Unmarshal(b, &q.data); err != nil {
			return nil, fmt.Errorf("review queue: parse %s: %w", path, err)
		}
	}

	return q, nil
}

// Add appends an item to the queue and persists it.
func (q *Queue) Add(item Item) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Deduplicate by Gmail message ID.
	for _, existing := range q.data {
		if existing.ID == item.ID {
			return nil
		}
	}

	if item.QueuedAt.IsZero() {
		item.QueuedAt = time.Now()
	}

	q.data = append(q.data, item)
	return q.save()
}

// List returns a snapshot of all pending review items, newest first.
func (q *Queue) List() []Item {
	q.mu.RLock()
	defer q.mu.RUnlock()

	out := make([]Item, len(q.data))
	// Reverse so newest items appear first.
	for i, item := range q.data {
		out[len(q.data)-1-i] = item
	}
	return out
}

// Get returns the item with the given Gmail message ID.
func (q *Queue) Get(id string) (Item, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for _, item := range q.data {
		if item.ID == id {
			return item, true
		}
	}
	return Item{}, false
}

// Remove deletes the item with the given Gmail message ID and persists.
func (q *Queue) Remove(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	updated := q.data[:0]
	for _, item := range q.data {
		if item.ID != id {
			updated = append(updated, item)
		}
	}
	q.data = updated
	return q.save()
}

func (q *Queue) save() error {
	b, err := json.MarshalIndent(q.data, "", "  ")
	if err != nil {
		return fmt.Errorf("review queue: marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(q.path), 0700); err != nil {
		return fmt.Errorf("review queue: mkdir: %w", err)
	}
	return os.WriteFile(q.path, b, 0600)
}
