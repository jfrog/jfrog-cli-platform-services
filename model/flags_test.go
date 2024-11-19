package model

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTimeoutParameter(t *testing.T) {
	tests := []struct {
		name         string
		flagProvider IntFlagProvider
		want         time.Duration
		wantErr      string
	}{
		{
			name:         "default",
			flagProvider: intFlagProviderStub{},
			want:         defaultTimeoutMillis * time.Millisecond,
		},
		{
			name:         "valid",
			flagProvider: intFlagProviderStub{FlagTimeout: {val: 150}},
			want:         150 * time.Millisecond,
		},
		{
			name:         "invalid",
			flagProvider: intFlagProviderStub{FlagTimeout: {err: errors.New("parse error")}},
			wantErr:      "invalid timeout provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTimeoutParameter(tt.flagProvider)
			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.Equalf(t, tt.want, got, "timeout mismatch")
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}

type intFlagProviderStub map[string]struct {
	val int
	err error
}

func (p intFlagProviderStub) IsFlagSet(name string) bool {
	_, isSet := p[name]
	return isSet
}

func (p intFlagProviderStub) GetIntFlagValue(name string) (int, error) {
	f, isSet := p[name]
	if isSet {
		return f.val, f.err
	}
	return 0, fmt.Errorf("flag %s used but not provided", name)
}
