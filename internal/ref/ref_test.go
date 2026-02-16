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

func TestContainsRef(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"ref://secrets/key", true},
		{"prefix ref://secrets/key suffix", true},
		{"postgres://ref://secrets/user:ref://secrets/pass@host", true},
		{"no refs here", false},
		{"", false},
		{"ref:/", false},
		{"ref:", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ContainsRef(tt.input)
			if got != tt.want {
				t.Errorf("ContainsRef(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindAll(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Embedded
	}{
		{
			name:  "no refs",
			input: "plain value",
			want:  nil,
		},
		{
			name:  "single top-level ref",
			input: "ref://secrets/api_key",
			want: []Embedded{
				{
					Ref:   Reference{Raw: "ref://secrets/api_key", Backend: "secrets", Path: "api_key"},
					Start: 0, End: 21,
				},
			},
		},
		{
			name:  "single embedded ref",
			input: "prefix=ref://keychain/db_pass&suffix",
			want: []Embedded{
				{
					Ref:   Reference{Raw: "ref://keychain/db_pass", Backend: "keychain", Path: "db_pass"},
					Start: 7, End: 29,
				},
			},
		},
		{
			name:  "two embedded refs",
			input: "postgres://ref://secrets/db_user:ref://secrets/db_pass@host",
			want: []Embedded{
				{
					Ref:   Reference{Raw: "ref://secrets/db_user", Backend: "secrets", Path: "db_user"},
					Start: 11, End: 32,
				},
				{
					Ref:   Reference{Raw: "ref://secrets/db_pass", Backend: "secrets", Path: "db_pass"},
					Start: 33, End: 54,
				},
			},
		},
		{
			name:  "ref delimited by special chars",
			input: "user=ref://secrets/user&pass=ref://secrets/pass",
			want: []Embedded{
				{
					Ref:   Reference{Raw: "ref://secrets/user", Backend: "secrets", Path: "user"},
					Start: 5, End: 23,
				},
				{
					Ref:   Reference{Raw: "ref://secrets/pass", Backend: "secrets", Path: "pass"},
					Start: 29, End: 47,
				},
			},
		},
		{
			name:  "ref with nested path",
			input: "value=ref://ssm/prod/db/password!end",
			want: []Embedded{
				{
					Ref:   Reference{Raw: "ref://ssm/prod/db/password", Backend: "ssm", Path: "prod/db/password"},
					Start: 6, End: 32,
				},
			},
		},
		{
			name:  "invalid ref skipped (no path)",
			input: "text ref://backend text",
			want:  nil,
		},
		{
			name:  "ref with hyphens and dots",
			input: "ref://my-backend/my.secret-key",
			want: []Embedded{
				{
					Ref:   Reference{Raw: "ref://my-backend/my.secret-key", Backend: "my-backend", Path: "my.secret-key"},
					Start: 0, End: 30,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindAll(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("FindAll(%q) returned %d results, want %d", tt.input, len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Ref.Raw != tt.want[i].Ref.Raw {
					t.Errorf("[%d] Ref.Raw = %q, want %q", i, got[i].Ref.Raw, tt.want[i].Ref.Raw)
				}
				if got[i].Ref.Backend != tt.want[i].Ref.Backend {
					t.Errorf("[%d] Ref.Backend = %q, want %q", i, got[i].Ref.Backend, tt.want[i].Ref.Backend)
				}
				if got[i].Ref.Path != tt.want[i].Ref.Path {
					t.Errorf("[%d] Ref.Path = %q, want %q", i, got[i].Ref.Path, tt.want[i].Ref.Path)
				}
				if got[i].Start != tt.want[i].Start {
					t.Errorf("[%d] Start = %d, want %d", i, got[i].Start, tt.want[i].Start)
				}
				if got[i].End != tt.want[i].End {
					t.Errorf("[%d] End = %d, want %d", i, got[i].End, tt.want[i].End)
				}
			}
		})
	}
}
