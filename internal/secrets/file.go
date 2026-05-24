package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type encryptedFileStore struct {
	mu      sync.RWMutex
	path    string
	gcm     cipher.AEAD
	secrets map[string]string
}

func newEncryptedFileStore(dataDir, hexKey string) (Store, error) {
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("secrets: decode encryption key: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, errors.New("secrets: AP5_ENCRYPTION_KEY must be a 64-char hex string (32 bytes / AES-256)")
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("secrets: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets: create GCM: %w", err)
	}

	storePath := filepath.Join(dataDir, "secrets", "store.enc")
	if err := os.MkdirAll(filepath.Dir(storePath), 0700); err != nil {
		return nil, fmt.Errorf("secrets: create store dir: %w", err)
	}

	s := &encryptedFileStore{
		path:    storePath,
		gcm:     gcm,
		secrets: make(map[string]string),
	}

	if err := s.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("secrets: load store: %w", err)
	}

	return s, nil
}

func (s *encryptedFileStore) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.secrets[key]
	if !ok {
		return "", ErrNotFound
	}
	return val, nil
}

func (s *encryptedFileStore) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.secrets[key] = value
	return s.save()
}

func (s *encryptedFileStore) load() error {
	ciphertext, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	if len(ciphertext) < s.gcm.NonceSize() {
		return errors.New("secrets: ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:s.gcm.NonceSize()], ciphertext[s.gcm.NonceSize():]
	plaintext, err := s.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("secrets: decrypt: %w", err)
	}

	return json.Unmarshal(plaintext, &s.secrets)
}

func (s *encryptedFileStore) save() error {
	plaintext, err := json.Marshal(s.secrets)
	if err != nil {
		return fmt.Errorf("secrets: marshal: %w", err)
	}

	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("secrets: generate nonce: %w", err)
	}

	ciphertext := s.gcm.Seal(nonce, nonce, plaintext, nil)
	return os.WriteFile(s.path, ciphertext, 0600)
}
