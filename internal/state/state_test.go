package state_test

import (
	"testing"

	"github.com/pushkar-anand/ap-5/internal/state"
)

func newStore(t *testing.T) *state.Store {
	t.Helper()
	s, err := state.New(t.TempDir())
	if err != nil {
		t.Fatalf("state.New: %v", err)
	}
	return s
}

func TestGetHistoryID_EmptyOnNew(t *testing.T) {
	s := newStore(t)
	if got := s.GetHistoryID("test@gmail.com"); got != "" {
		t.Errorf("GetHistoryID on new store = %q, want empty", got)
	}
}

func TestSetAndGetHistoryID(t *testing.T) {
	s := newStore(t)

	if err := s.SetHistoryID("test@gmail.com", "12345"); err != nil {
		t.Fatalf("SetHistoryID: %v", err)
	}

	got := s.GetHistoryID("test@gmail.com")
	if got != "12345" {
		t.Errorf("GetHistoryID = %q, want %q", got, "12345")
	}
}

func TestSetHistoryID_MultipleAccounts(t *testing.T) {
	s := newStore(t)

	_ = s.SetHistoryID("a@gmail.com", "111")
	_ = s.SetHistoryID("b@gmail.com", "222")

	if got := s.GetHistoryID("a@gmail.com"); got != "111" {
		t.Errorf("a: got %q, want %q", got, "111")
	}
	if got := s.GetHistoryID("b@gmail.com"); got != "222" {
		t.Errorf("b: got %q, want %q", got, "222")
	}
}

func TestSetHistoryID_PersistsAcrossReloads(t *testing.T) {
	dir := t.TempDir()

	s1, _ := state.New(dir)
	_ = s1.SetHistoryID("user@gmail.com", "99999")

	s2, err := state.New(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	got := s2.GetHistoryID("user@gmail.com")
	if got != "99999" {
		t.Errorf("after reload: got %q, want %q", got, "99999")
	}
}

func TestSetHistoryID_Overwrite(t *testing.T) {
	s := newStore(t)

	_ = s.SetHistoryID("user@gmail.com", "old")
	_ = s.SetHistoryID("user@gmail.com", "new")

	if got := s.GetHistoryID("user@gmail.com"); got != "new" {
		t.Errorf("after overwrite: got %q, want %q", got, "new")
	}
}
