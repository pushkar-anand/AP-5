package secrets

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const keyringService = "ap5"

type keyringStore struct{}

func newKeyringStore() Store {
	return &keyringStore{}
}

func (s *keyringStore) Get(key string) (string, error) {
	val, err := keyring.Get(keyringService, key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("keyring get %q: %w", key, err)
	}
	return val, nil
}

func (s *keyringStore) Set(key, value string) error {
	if err := keyring.Set(keyringService, key, value); err != nil {
		return fmt.Errorf("keyring set %q: %w", key, err)
	}
	return nil
}
