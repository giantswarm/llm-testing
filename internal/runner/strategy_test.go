package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		wantName string
		wantErr  bool
	}{
		{"qa strategy", "qa", "qa", false},
		{"empty defaults to qa", "", "qa", false},
		{"unknown strategy", "tool-use", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := GetStrategy(tt.strategy)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, s.Name())
		})
	}
}
