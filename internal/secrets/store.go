package secrets

import (
	"errors"
	"os"
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

func NewStore(dataDir string) (Store, error) {
	backend := strings.ToLower(os.Getenv("AP5_SECRET_BACKEND"))
	if backend == "" {
		backend = BackendKeyring
	}

	switch backend {
	case BackendFile:
		encKey := os.Getenv("AP5_ENCRYPTION_KEY")
		if encKey == "" {
			return nil, errors.New("secrets: AP5_ENCRYPTION_KEY must be set when using file backend")
		}
		return newEncryptedFileStore(dataDir, encKey)
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
