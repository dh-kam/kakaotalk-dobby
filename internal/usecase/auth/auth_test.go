package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanAuthorizationCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace string",
			input:    "   \n\t  ",
			expected: "",
		},
		{
			name:     "raw authorization code without URL",
			input:    "msAuSwFb2pryPWTVOfqXKKbWx_tsWhd8McuQHFDHSB0APmpionmisgAAAAQKDSAbAAABoUy1tEbE017PSiBv1Q",
			expected: "msAuSwFb2pryPWTVOfqXKKbWx_tsWhd8McuQHFDHSB0APmpionmisgAAAAQKDSAbAAABoUy1tEbE017PSiBv1Q",
		},
		{
			name:     "raw authorization code with surrounding whitespace",
			input:    "  msAuSwFb2pryPWTVOfqXKKbWx_tsWhd8McuQHFDHSB0APmpionmisgAAAAQKDSAbAAABoUy1tEbE017PSiBv1Q \n ",
			expected: "msAuSwFb2pryPWTVOfqXKKbWx_tsWhd8McuQHFDHSB0APmpionmisgAAAAQKDSAbAAABoUy1tEbE017PSiBv1Q",
		},
		{
			name:     "full redirect URL with http scheme and port",
			input:    "http://localhost:8080/callback?code=msAuSwFb2pryPWTVOfqXKKbWx_tsWhd8McuQHFDHSB0APmpionmisgAAAAQKDSAbAAABoUy1tEbE017PSiBv1Q",
			expected: "msAuSwFb2pryPWTVOfqXKKbWx_tsWhd8McuQHFDHSB0APmpionmisgAAAAQKDSAbAAABoUy1tEbE017PSiBv1Q",
		},
		{
			name:     "full redirect URL with https scheme and multiple query params",
			input:    "https://auth.example.com/callback?state=xyz123&code=sample_auth_code_999&scope=talk_message",
			expected: "sample_auth_code_999",
		},
		{
			name:     "redirect URL without scheme",
			input:    "localhost:8080/callback?code=sample_code_no_scheme",
			expected: "sample_code_no_scheme",
		},
		{
			name:     "path-only query string",
			input:    "/callback?code=sample_code_path_only",
			expected: "sample_code_path_only",
		},
		{
			name:     "query-only string",
			input:    "?code=sample_code_query_only",
			expected: "sample_code_query_only",
		},
		{
			name:     "key-value only string",
			input:    "code=sample_code_kv_only",
			expected: "sample_code_kv_only",
		},
		{
			name:     "url with fragment anchor",
			input:    "http://localhost:8080/callback?code=sample_code_with_hash#section",
			expected: "sample_code_with_hash",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := CleanAuthorizationCode(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
