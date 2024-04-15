package model

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"golang.org/x/crypto/scrypt"
)

const (
	minPasswordLength   = 12
	encryptionKeyLength = 32
)

type Secret struct {
	Key              string `json:"key"`
	Value            string `json:"value"`
	MarkedForRemoval bool   `json:"markedForRemoval"`
}

func ReadSecretPassword(prompt ...string) (string, error) {
	passwordFromEnv, passwordInEnv := os.LookupEnv(EnvKeySecretsPassword)
	if passwordInEnv {
		return passwordFromEnv, nil
	}

	message := "Password: "
	if len(prompt) > 0 {
		message = prompt[0]
	}

	password, err := ioutils.ScanPasswordFromConsole(message)
	if err != nil {
		return "", err
	}

	err = validateSecretPassword(password)
	if err == nil {
		return password, nil
	}

	return "", err
}

func EncryptSecret(password string, secretValue string) (string, error) {
	encryptionKey, salt, err := deriveKey([]byte(password), nil)
	if err != nil {
		return "", err
	}

	blockCipher, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(blockCipher)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}

	cipherBytes := gcm.Seal(nonce, nonce, []byte(secretValue), nil)

	cipherBytes = append(cipherBytes, salt...)

	return base64.StdEncoding.EncodeToString(cipherBytes), nil
}

func DecryptSecret(password string, encryptedValue string) (string, error) {
	encryptedBytes, err := base64.StdEncoding.DecodeString(encryptedValue)
	if err != nil {
		return "", err
	}

	if len(encryptedBytes) < encryptionKeyLength {
		return "", errors.New("invalid encrypted secret length")
	}

	salt, data := encryptedBytes[len(encryptedBytes)-encryptionKeyLength:], encryptedBytes[:len(encryptedBytes)-encryptionKeyLength]

	encryptionKey, _, err := deriveKey([]byte(password), salt)
	if err != nil {
		return "", err
	}

	blockCipher, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(blockCipher)
	if err != nil {
		return "", err
	}

	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]

	clearTextBytes, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(clearTextBytes), nil
}

// deriveKey Create a 32-bit key from any password. Needed to use AES
func deriveKey(password, salt []byte) ([]byte, []byte, error) {
	if salt == nil {
		salt = make([]byte, encryptionKeyLength)
		if _, err := rand.Read(salt); err != nil {
			return nil, nil, err
		}
	}

	key, err := scrypt.Key(password, salt, 16384, 8, 1, encryptionKeyLength)
	if err != nil {
		return nil, nil, err
	}

	return key, salt, nil
}

func validateSecretPassword(key string) error {
	if len(key) < minPasswordLength {
		return fmt.Errorf("a secret should have a minimum length of %d, got %d", minPasswordLength, len(key))
	}
	return nil
}
