package ref

import (
	"testing"
)

func TestIsRef(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"ref://secrets/api_key", true},
		{"ref://keychain/db_pass", true},
		{"ref://ssm/prod/db/password", true},
		{"ref://", true}, // has prefix, even if invalid for Parse
		{"ref://backend/", true},
		{"plaintext-value", false},
		{"", false},
		{"REF://secrets/key", false}, // case-sensitive
		{"http://example.com", false},
		{"ref:/secrets/key", false}, // single slash
		{"reference://secrets/key", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := IsRef(tt.value)
			if got != tt.want {
				t.Errorf("IsRef(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantBackend string
		wantPath    string
		wantErr     bool
	}{
		{
			name:        "secrets backend",
			input:       "ref://secrets/api_key",
			wantBackend: "secrets",
			wantPath:    "api_key",
		},
		{
			name:        "keychain backend",
			input:       "ref://keychain/db_pass",
			wantBackend: "keychain",
			wantPath:    "db_pass",
		},
		{
			name:        "ssm backend with nested path",
			input:       "ref://ssm/prod/db/password",
			wantBackend: "ssm",
			wantPath:    "prod/db/password",
		},
		{
			name:        "vault backend with deep path",
			input:       "ref://vault/secret/data/myapp/db_password",
			wantBackend: "vault",
			wantPath:    "secret/data/myapp/db_password",
		},
		{
			name:        "1password backend",
			input:       "ref://1password/my-vault/api-key",
			wantBackend: "1password",
			wantPath:    "my-vault/api-key",
		},
		{
			name:    "not a ref URI",
			input:   "plaintext-value",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "empty after prefix",
			input:   "ref://",
			wantErr: true,
		},
		{
			name:    "backend only, no path",
			input:   "ref://secrets",
			wantErr: true,
		},
		{
			name:    "backend with trailing slash, empty path",
			input:   "ref://secrets/",
			wantErr: true,
		},
		{
			name:    "empty backend",
			input:   "ref:///path",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result: %+v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Backend != tt.wantBackend {
				t.Errorf("Backend: got %q, want %q", got.Backend, tt.wantBackend)
			}
			if got.Path != tt.wantPath {
				t.Errorf("Path: got %q, want %q", got.Path, tt.wantPath)
			}
			if got.Raw != tt.input {
				t.Errorf("Raw: got %q, want %q", got.Raw, tt.input)
			}
		})
	}
}

func TestReferenceString(t *testing.T) {
	ref := Reference{
		Raw:     "ref://secrets/api_key",
		Backend: "secrets",
		Path:    "api_key",
	}
	got := ref.String()
	want := "ref://secrets/api_key"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestReferenceStringNestedPath(t *testing.T) {
	ref := Reference{
		Raw:     "ref://ssm/prod/db/password",
		Backend: "ssm",
		Path:    "prod/db/password",
	}
	got := ref.String()
	want := "ref://ssm/prod/db/password"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
