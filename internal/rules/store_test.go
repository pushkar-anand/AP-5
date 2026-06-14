package rules_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pushkar-anand/ap-5/internal/rules"
)

func newStore(t *testing.T) *rules.Store {
	t.Helper()
	s, err := rules.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func rule(category string) rules.Rule {
	return rules.Rule{
		Category:         category,
		ExtractionPrompt: "Extract details for " + category,
		Action:           rules.ActionImportTransaction,
	}
}

func TestStore_SaveAndList(t *testing.T) {
	s := newStore(t)

	if err := s.Save(rule("bank_account_transaction")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.Save(rule("bill_payment")); err != nil {
		t.Fatalf("Save: %v", err)
	}

	list := s.List()
	if len(list) != 2 {
		t.Fatalf("List len = %d, want 2", len(list))
	}
}

func TestStore_SaveUpdatesExistingCategory(t *testing.T) {
	s := newStore(t)

	_ = s.Save(rule("bank_account_transaction"))

	updated := rule("bank_account_transaction")
	updated.Action = rules.ActionLogOnly
	_ = s.Save(updated)

	list := s.List()
	if len(list) != 1 {
		t.Fatalf("List len = %d, want 1 after update", len(list))
	}
	if list[0].Action != rules.ActionLogOnly {
		t.Errorf("Action = %q, want log_only", list[0].Action)
	}
}

func TestStore_Get(t *testing.T) {
	s := newStore(t)
	_ = s.Save(rule("bill_payment"))

	got, ok := s.Get("bill_payment")
	if !ok {
		t.Fatal("Get returned not-found for existing rule")
	}
	if got.Category != "bill_payment" {
		t.Errorf("Category = %q, want bill_payment", got.Category)
	}

	_, ok = s.Get("nonexistent")
	if ok {
		t.Error("Get returned found for missing rule")
	}
}

func TestStore_Delete(t *testing.T) {
	s := newStore(t)
	_ = s.Save(rule("bank_account_transaction"))
	_ = s.Save(rule("bill_payment"))

	if err := s.Delete("bank_account_transaction"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	list := s.List()
	if len(list) != 1 {
		t.Fatalf("List len after delete = %d, want 1", len(list))
	}
	if list[0].Category != "bill_payment" {
		t.Errorf("remaining rule = %q, want bill_payment", list[0].Category)
	}

	// Deleting nonexistent is a no-op.
	if err := s.Delete("nonexistent"); err != nil {
		t.Errorf("Delete nonexistent: %v", err)
	}
}

func TestStore_PersistsAcrossReloads(t *testing.T) {
	dir := t.TempDir()

	s1, _ := rules.NewStore(dir)
	_ = s1.Save(rule("persisted_rule"))

	s2, err := rules.NewStore(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	list := s2.List()
	if len(list) != 1 {
		t.Fatalf("after reload, List len = %d, want 1", len(list))
	}
	if list[0].Category != "persisted_rule" {
		t.Errorf("reloaded category = %q, want persisted_rule", list[0].Category)
	}
}

func TestStore_FileIsJSON(t *testing.T) {
	dir := t.TempDir()
	s, _ := rules.NewStore(dir)
	_ = s.Save(rule("json_check"))

	b, err := os.ReadFile(filepath.Join(dir, "learned_rules.json"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var arr []rules.Rule
	if err := json.Unmarshal(b, &arr); err != nil {
		t.Errorf("file is not valid JSON: %v", err)
	}
}
