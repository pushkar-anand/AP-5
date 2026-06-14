package review_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pushkar-anand/ap-5/internal/review"
)

func newQueue(t *testing.T) *review.Queue {
	t.Helper()
	q, err := review.NewQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	return q
}

func item(id string) review.Item {
	return review.Item{
		ID:            id,
		Account:       "user@example.com",
		Subject:       "Test subject " + id,
		Body:          "Test body",
		SuggestedType: "other",
		QueuedAt:      time.Now(),
	}
}

func TestQueue_AddAndList(t *testing.T) {
	q := newQueue(t)

	if err := q.Add(item("msg1")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := q.Add(item("msg2")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	items := q.List()
	if len(items) != 2 {
		t.Fatalf("List len = %d, want 2", len(items))
	}
	// Newest first: msg2 should be index 0.
	if items[0].ID != "msg2" {
		t.Errorf("items[0].ID = %q, want msg2", items[0].ID)
	}
	if items[1].ID != "msg1" {
		t.Errorf("items[1].ID = %q, want msg1", items[1].ID)
	}
}

func TestQueue_AddDeduplicates(t *testing.T) {
	q := newQueue(t)

	if err := q.Add(item("msg1")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := q.Add(item("msg1")); err != nil {
		t.Fatalf("Add duplicate: %v", err)
	}

	if got := len(q.List()); got != 1 {
		t.Errorf("List len after duplicate add = %d, want 1", got)
	}
}

func TestQueue_Get(t *testing.T) {
	q := newQueue(t)
	_ = q.Add(item("abc"))

	got, ok := q.Get("abc")
	if !ok {
		t.Fatal("Get returned not-found for existing item")
	}
	if got.ID != "abc" {
		t.Errorf("Get.ID = %q, want abc", got.ID)
	}

	_, ok = q.Get("nonexistent")
	if ok {
		t.Error("Get returned found for missing item")
	}
}

func TestQueue_Remove(t *testing.T) {
	q := newQueue(t)
	_ = q.Add(item("msg1"))
	_ = q.Add(item("msg2"))

	if err := q.Remove("msg1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	items := q.List()
	if len(items) != 1 {
		t.Fatalf("List len after remove = %d, want 1", len(items))
	}
	if items[0].ID != "msg2" {
		t.Errorf("remaining item ID = %q, want msg2", items[0].ID)
	}

	// Removing nonexistent ID is a no-op.
	if err := q.Remove("nonexistent"); err != nil {
		t.Errorf("Remove nonexistent: %v", err)
	}
}

func TestQueue_PersistsAcrossReloads(t *testing.T) {
	dir := t.TempDir()

	q1, _ := review.NewQueue(dir)
	_ = q1.Add(item("persisted"))

	q2, err := review.NewQueue(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	items := q2.List()
	if len(items) != 1 {
		t.Fatalf("after reload, List len = %d, want 1", len(items))
	}
	if items[0].ID != "persisted" {
		t.Errorf("reloaded item ID = %q, want persisted", items[0].ID)
	}
}

func TestQueue_FileIsJSON(t *testing.T) {
	dir := t.TempDir()
	q, _ := review.NewQueue(dir)
	_ = q.Add(item("json-check"))

	b, err := os.ReadFile(filepath.Join(dir, "review_queue.json"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var arr []review.Item
	if err := json.Unmarshal(b, &arr); err != nil {
		t.Errorf("file is not valid JSON: %v", err)
	}
}
