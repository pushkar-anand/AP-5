package secrets

import (
	"errors"
	"strings"
)

var ErrNotFound = errors.New("secret not found")

type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

const (
	BackendKeyring = "keyring"
	BackendFile    = "file"
)

func NewStore(dataDir, backend, encryptionKey string) (Store, error) {
	switch strings.ToLower(backend) {
	case BackendFile:
		if encryptionKey == "" {
			return nil, errors.New("secrets: encryption_key must be set in config when using file backend")
		}
		return newEncryptedFileStore(dataDir, encryptionKey)
	default:
		return newKeyringStore(), nil
	}
}

// GmailTokenKey returns the secret store key for a Gmail OAuth token.
func GmailTokenKey(email string) string {
	return "ap5/gmail/" + email
}

// JN66TokenKey returns the secret store key for a JN-66 bearer token.
func JN66TokenKey(email string) string {
	return "ap5/jn66/" + email
}
