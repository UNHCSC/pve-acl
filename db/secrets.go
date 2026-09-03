package db

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/z46-dev/gosqlite"
)

type SecretCreateInput struct {
	Name       string
	SecretType SecretType
	Value      string
	OwnerType  SecretOwnerType
	OwnerID    *int
}

var (
	secretCipherMu        sync.RWMutex
	secretCipher          cipher.AEAD
	sensitiveValuePattern = regexp.MustCompile(`(?i)(password|secret|token|credential)(\s*[:=]\s*)[^\s,;]+`)
)

// ConfigureSecretEncryption installs a deployment-provided 256-bit base64 key.
func ConfigureSecretEncryption(encodedKey string) (errResult error) {
	encodedKey = strings.TrimSpace(encodedKey)
	if encodedKey == "" {
		secretCipherMu.Lock()
		secretCipher = nil
		secretCipherMu.Unlock()
		return nil
	}
	var key []byte

	key, errResult = base64.StdEncoding.DecodeString(encodedKey)
	if errResult != nil || len(key) != 32 {
		return fmt.Errorf("secret master key must be base64-encoded 32 bytes")
	}
	var block cipher.Block

	block, errResult = aes.NewCipher(key)
	if errResult != nil {
		return errResult
	}
	var configured cipher.AEAD

	configured, errResult = cipher.NewGCM(block)
	if errResult != nil {
		return errResult
	}
	secretCipherMu.Lock()
	secretCipher = configured
	secretCipherMu.Unlock()
	return nil
}

// SecretEncryptionConfigured reports whether live secret-backed operations are safe.
func SecretEncryptionConfigured() bool {
	secretCipherMu.RLock()
	defer secretCipherMu.RUnlock()
	return secretCipher != nil
}

// RedactSensitiveText removes common credential assignments from logs and errors.
func RedactSensitiveText(value string) (valueResult string) {
	return sensitiveValuePattern.ReplaceAllString(value, "$1$2[REDACTED]")
}

// CreateSecret encrypts and stores a secret without exposing its value.
func CreateSecret(input SecretCreateInput) (secretResult *Secret, errResult error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return nil, fmt.Errorf("secret name is required")
	}
	if input.Value == "" {
		return nil, fmt.Errorf("secret value is required")
	}
	var encrypted []byte

	encrypted, errResult = encryptSecret([]byte(input.Value))
	if errResult != nil {
		return nil, errResult
	}
	var uuid string

	uuid, errResult = randomUUID()
	if errResult != nil {
		return nil, errResult
	}
	var now time.Time = time.Now().UTC()
	var secret *Secret = &Secret{UUID: uuid, Name: input.Name, SecretType: input.SecretType, EncryptedValue: encrypted, OwnerType: input.OwnerType, OwnerID: input.OwnerID, CreatedAt: now, UpdatedAt: now}
	if errResult = Secrets.Insert(secret); errResult != nil {
		return nil, errResult
	}
	return secret, nil
}

// RotateSecret replaces the encrypted value for an active secret.
func RotateSecret(secret *Secret, value string) (errResult error) {
	if secret == nil || secret.ArchivedAt != nil {
		return fmt.Errorf("secret was not found")
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("secret value is required")
	}
	secret.EncryptedValue, errResult = encryptSecret([]byte(value))
	if errResult != nil {
		return errResult
	}
	secret.UpdatedAt = time.Now().UTC()
	return Secrets.Update(secret)
}

// ReadSecret decrypts a secret for internal service use only.
func ReadSecret(secret *Secret) (valueResult string, errResult error) {
	if secret == nil || secret.ArchivedAt != nil {
		return "", fmt.Errorf("secret was not found")
	}
	var plaintext []byte

	plaintext, errResult = decryptSecret(secret.EncryptedValue)
	if errResult != nil {
		return "", errResult
	}
	return string(plaintext), nil
}

// ArchiveSecret archives a secret and clears its ciphertext.
func ArchiveSecret(secret *Secret) (errResult error) {
	if secret == nil || secret.ArchivedAt != nil {
		return fmt.Errorf("secret was not found")
	}
	var now time.Time = time.Now().UTC()
	secret.ArchivedAt, secret.UpdatedAt, secret.EncryptedValue = &now, now, []byte{}
	return Secrets.Update(secret)
}

// ActiveSecrets lists secret metadata without decrypted values.
func ActiveSecrets() (itemsResult []*Secret, errResult error) {
	return Secrets.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(Secrets.FieldBySQLName("archived_at"), gosqlite.OpIsNull, nil))
}

func encryptSecret(plaintext []byte) (result []byte, errResult error) {
	secretCipherMu.RLock()
	defer secretCipherMu.RUnlock()
	if secretCipher == nil {
		return nil, fmt.Errorf("secret encryption is not configured")
	}
	var nonce []byte = make([]byte, secretCipher.NonceSize())
	if _, errResult = io.ReadFull(rand.Reader, nonce); errResult != nil {
		return nil, errResult
	}
	return secretCipher.Seal(nonce, nonce, plaintext, nil), nil
}

func decryptSecret(encrypted []byte) (result []byte, errResult error) {
	secretCipherMu.RLock()
	defer secretCipherMu.RUnlock()
	if secretCipher == nil {
		return nil, fmt.Errorf("secret encryption is not configured")
	}
	if len(encrypted) < secretCipher.NonceSize() {
		return nil, fmt.Errorf("encrypted secret is invalid")
	}
	return secretCipher.Open(nil, encrypted[:secretCipher.NonceSize()], encrypted[secretCipher.NonceSize():], nil)
}
