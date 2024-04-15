package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryption(t *testing.T) {
	const secretValue = "my-secret-value"
	const password = "my-password"

	encrypted, err := EncryptSecret(password, secretValue)
	require.NoError(t, err)
	require.NotEqual(t, secretValue, encrypted)

	decrypted, err := DecryptSecret(password, encrypted)
	require.NoError(t, err)

	assert.Equal(t, secretValue, decrypted)
}

func Test_validateSecretPassword(t *testing.T) {
	tests := []struct {
		name            string
		password        string
		validationError string
	}{
		{
			name:     "valid",
			password: "/abcdef123456!",
		},
		{
			name:            "too short",
			password:        "abcdef",
			validationError: fmt.Sprintf("a secret should have a minimum length of %d, got 6", minPasswordLength),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSecretPassword(tt.password)
			if tt.validationError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.validationError)
			}
		})
	}
}
