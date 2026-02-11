package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateHTTPSRequirement(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{
			name:    "https is valid",
			baseURL: "https://llm-testing.example.com",
			wantErr: false,
		},
		{
			name:    "localhost http is valid",
			baseURL: "http://localhost:8080",
			wantErr: false,
		},
		{
			name:    "127.0.0.1 http is valid",
			baseURL: "http://127.0.0.1:8080",
			wantErr: false,
		},
		{
			name:    "ipv6 loopback http is valid",
			baseURL: "http://[::1]:8080",
			wantErr: false,
		},
		{
			name:    "non-localhost http is invalid",
			baseURL: "http://example.com",
			wantErr: true,
		},
		{
			name:    "empty URL is invalid",
			baseURL: "",
			wantErr: true,
		},
		{
			name:    "ftp scheme is invalid",
			baseURL: "ftp://example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHTTPSRequirement(tt.baseURL)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
