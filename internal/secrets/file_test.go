package secrets

import (
	"errors"
	"testing"
)

func validKey() string {
	// 32 zero bytes encoded as 64 hex chars
	return "0000000000000000000000000000000000000000000000000000000000000000"
}

func newTestFileStore(t *testing.T) Store {
	t.Helper()
	s, err := newEncryptedFileStore(t.TempDir(), validKey())
	if err != nil {
		t.Fatalf("newEncryptedFileStore: %v", err)
	}
	return s
}

func TestEncryptedFileStore_SetAndGet(t *testing.T) {
	s := newTestFileStore(t)

	if err := s.Set("mykey", "myvalue"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := s.Get("mykey")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "myvalue" {
		t.Errorf("Get = %q, want %q", got, "myvalue")
	}
}

func TestEncryptedFileStore_NotFound(t *testing.T) {
	s := newTestFileStore(t)

	_, err := s.Get("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get missing key: got %v, want ErrNotFound", err)
	}
}

func TestEncryptedFileStore_Overwrite(t *testing.T) {
	s := newTestFileStore(t)

	_ = s.Set("k", "first")
	_ = s.Set("k", "second")

	got, err := s.Get("k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "second" {
		t.Errorf("Get after overwrite = %q, want %q", got, "second")
	}
}

func TestEncryptedFileStore_MultipleKeys(t *testing.T) {
	s := newTestFileStore(t)

	_ = s.Set("a", "alpha")
	_ = s.Set("b", "beta")

	a, _ := s.Get("a")
	b, _ := s.Get("b")
	if a != "alpha" || b != "beta" {
		t.Errorf("a=%q b=%q", a, b)
	}
}

func TestEncryptedFileStore_PersistsAcrossReloads(t *testing.T) {
	dir := t.TempDir()
	key := validKey()

	s1, _ := newEncryptedFileStore(dir, key)
	_ = s1.Set("persistent", "value123")

	s2, err := newEncryptedFileStore(dir, key)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	got, err := s2.Get("persistent")
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}
	if got != "value123" {
		t.Errorf("after reload: got %q, want %q", got, "value123")
	}
}

func TestEncryptedFileStore_InvalidKey(t *testing.T) {
	_, err := newEncryptedFileStore(t.TempDir(), "tooshort")
	if err == nil {
		t.Error("expected error for invalid key length, got nil")
	}
}

func TestEncryptedFileStore_InvalidHexKey(t *testing.T) {
	_, err := newEncryptedFileStore(t.TempDir(), "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz")
	if err == nil {
		t.Error("expected error for invalid hex key, got nil")
	}
}
