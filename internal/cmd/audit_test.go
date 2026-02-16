package cmd

import (
	"bytes"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditCmd_NoSecrets(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\nDB_PORT=5432\nDEBUG=true\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestAuditCmd_NoSecrets_Quiet(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"--quiet", "audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "" {
		t.Errorf("expected no output in quiet mode, got %q", buf.String())
	}
}

func TestAuditCmd_StripeKey(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env",
		"STRIPE_KEY=sk_live_1234567890abcdef1234567890\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for Stripe key, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "STRIPE_KEY") {
		t.Errorf("expected STRIPE_KEY in output, got %q", stderr)
	}
	if !strings.Contains(stderr, "Stripe key") {
		t.Errorf("expected Stripe pattern name, got %q", stderr)
	}
}

func TestAuditCmd_GitHubToken(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env",
		"GH_TOKEN=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for GitHub token, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "GH_TOKEN") {
		t.Errorf("expected GH_TOKEN in output, got %q", stderr)
	}
	if !strings.Contains(stderr, "GitHub token") {
		t.Errorf("expected GitHub pattern name, got %q", stderr)
	}
}

func TestAuditCmd_AWSAccessKey(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env",
		"AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for AWS key, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "AWS_ACCESS_KEY_ID") {
		t.Errorf("expected AWS_ACCESS_KEY_ID in output, got %q", stderr)
	}
	if !strings.Contains(stderr, "AWS access key") {
		t.Errorf("expected AWS pattern name, got %q", stderr)
	}
}

func TestAuditCmd_JWTToken(t *testing.T) {
	dir := t.TempDir()
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	envPath := writeTestFile(t, dir, ".env",
		"AUTH_TOKEN="+jwt+"\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for JWT, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "AUTH_TOKEN") {
		t.Errorf("expected AUTH_TOKEN in output, got %q", stderr)
	}
}

func TestAuditCmd_SecretKeyName(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env",
		"DATABASE_PASSWORD=mysuperpassword123\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for password in key name, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "DATABASE_PASSWORD") {
		t.Errorf("expected DATABASE_PASSWORD in output, got %q", stderr)
	}
	if !strings.Contains(stderr, "key name suggests a secret") {
		t.Errorf("expected key-name detection reason, got %q", stderr)
	}
}

func TestAuditCmd_HighEntropy(t *testing.T) {
	dir := t.TempDir()
	// High-entropy random-looking string.
	envPath := writeTestFile(t, dir, ".env",
		"SOME_VAR=a8Kp2xR9mQ4wZ7vN3yB6cJ1dF5hL0tGsEiUo\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for high-entropy value, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "SOME_VAR") {
		t.Errorf("expected SOME_VAR in output, got %q", stderr)
	}
	if !strings.Contains(stderr, "high-entropy") {
		t.Errorf("expected entropy detection reason, got %q", stderr)
	}
}

func TestAuditCmd_RefReferenceSafe(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env",
		"API_SECRET=ref://secrets/api_secret\nDB_HOST=localhost\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("ref:// values should not be flagged, got error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestAuditCmd_InterpolationSafe(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env",
		"DB_URL=postgres://${DB_USER}:${DB_PASS}@localhost/app\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("interpolation values should not be flagged, got error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestAuditCmd_ShortPasswordSafe(t *testing.T) {
	dir := t.TempDir()
	// Key name suggests secret but value is too short (< 8 chars).
	envPath := writeTestFile(t, dir, ".env",
		"API_KEY=dev\nDB_HOST=localhost\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("short values for secret key names should not be flagged, got error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestAuditCmd_BothFilesChecked(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")
	localPath := writeTestFile(t, dir, ".env.local",
		"MY_SECRET=sk_live_1234567890abcdef1234567890\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", localPath,
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for secret in .env.local, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "MY_SECRET") {
		t.Errorf("expected MY_SECRET from .env.local, got %q", stderr)
	}
}

func TestAuditCmd_MultipleSecrets(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env",
		"STRIPE_KEY=sk_live_1234567890abcdef1234567890\nGH_TOKEN=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij\nDB_HOST=localhost\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for multiple secrets, got nil")
	}

	if !strings.Contains(err.Error(), "2 potential secret(s) found") {
		t.Errorf("expected 2 secrets, got error: %v", err)
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "STRIPE_KEY") {
		t.Errorf("expected STRIPE_KEY in output, got %q", stderr)
	}
	if !strings.Contains(stderr, "GH_TOKEN") {
		t.Errorf("expected GH_TOKEN in output, got %q", stderr)
	}
}

func TestAuditCmd_MinEntropyFlag(t *testing.T) {
	dir := t.TempDir()
	// This value has moderate entropy. With a high threshold, it should pass.
	envPath := writeTestFile(t, dir, ".env",
		"SOME_CONFIG=aabbccddeeffgghhiijj\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
		"--min-entropy", "5.0",
	})

	// With a very high entropy threshold, this moderate-entropy value should pass.
	if err := root.Execute(); err != nil {
		t.Fatalf("high entropy threshold should skip moderate values, got error: %v", err)
	}
}

func TestAuditCmd_NoFiles(t *testing.T) {
	dir := t.TempDir()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"audit",
		"--file", filepath.Join(dir, ".env"),
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("no files should be OK, got error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestAuditCmd_RejectsArguments(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"audit", "extra-arg"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for unexpected argument, got nil")
	}
}

func TestAuditCmd_PrivateKeyHeader(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env",
		"CERT_KEY=-----BEGIN PRIVATE KEY-----\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for private key header, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "CERT_KEY") {
		t.Errorf("expected CERT_KEY in output, got %q", stderr)
	}
	if !strings.Contains(stderr, "private key header") {
		t.Errorf("expected private key pattern name, got %q", stderr)
	}
}

func TestAuditCmd_SlackToken(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env",
		"SLACK_TOKEN=xoxb-123456789012-123456789012-abcdefghijklmnop\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for Slack token, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "SLACK_TOKEN") {
		t.Errorf("expected SLACK_TOKEN in output, got %q", stderr)
	}
}

func TestAuditCmd_EmptyValueSafe(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env",
		"API_SECRET=\nDB_HOST=localhost\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("empty values should not be flagged, got error: %v", err)
	}
}

func TestAuditCmd_HintMessage(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env",
		"STRIPE_KEY=sk_live_1234567890abcdef1234567890\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"audit",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	_ = root.Execute()

	stderr := errBuf.String()
	if !strings.Contains(stderr, "ref://") {
		t.Errorf("expected ref:// hint in output, got %q", stderr)
	}
}

func TestShannonEntropy(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMin float64
		wantMax float64
	}{
		{"empty string", "", 0, 0},
		{"single char", "aaaa", 0, 0.01},
		{"two chars equal", "abababab", 0.99, 1.01},
		{"high entropy", "aB3$xY9!mK7@pQ2&", 3.5, 5.0},
		{"hex string", "0123456789abcdef", 3.9, 4.1},
		{"all same", "zzzzzzzzz", 0, 0.01},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shannonEntropy(tt.input)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("shannonEntropy(%q) = %f, want between %f and %f",
					tt.input, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestShannonEntropy_Precision(t *testing.T) {
	// For a string with all unique characters, entropy = log2(n)
	s := "abcdefghijklmnop" // 16 unique chars
	expected := math.Log2(16)
	got := shannonEntropy(s)
	if math.Abs(got-expected) > 0.01 {
		t.Errorf("shannonEntropy(%q) = %f, expected %f", s, got, expected)
	}
}

func TestMatchesKnownSecretPattern(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantMatch bool
		wantName  string
	}{
		{"stripe live key", "sk_live_1234567890abcdef12", true, "Stripe key"},
		{"stripe test key", "sk_test_1234567890abcdef12", true, "Stripe key"},
		{"github pat", "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij", true, "GitHub token"},
		{"aws key", "AKIAIOSFODNN7EXAMPLE", true, "AWS access key"},
		{"not a key", "hello-world", false, ""},
		{"short value", "sk_", false, ""},
		{"localhost", "localhost", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, matched := matchesKnownSecretPattern(tt.value)
			if matched != tt.wantMatch {
				t.Errorf("matchesKnownSecretPattern(%q) matched=%v, want %v",
					tt.value, matched, tt.wantMatch)
			}
			if matched && name != tt.wantName {
				t.Errorf("matchesKnownSecretPattern(%q) name=%q, want %q",
					tt.value, name, tt.wantName)
			}
		})
	}
}

func TestIsSecretKeyName(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"API_KEY", true},
		{"DATABASE_PASSWORD", true},
		{"MY_SECRET", true},
		{"AUTH_TOKEN", true},
		{"CLIENT_SECRET", true},
		{"PRIVATE_KEY", true},
		{"ACCESS_KEY_ID", true},
		{"DB_HOST", false},
		{"APP_NAME", false},
		{"PORT", false},
		{"DEBUG", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := isSecretKeyName(tt.key)
			if got != tt.want {
				t.Errorf("isSecretKeyName(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestIsSafeValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"true", "true", true},
		{"false", "false", true},
		{"localhost", "localhost", true},
		{"short", "dev", true},
		{"number", "3000", true},
		{"ip4", "127.0.0.1", true},
		{"http localhost", "http://localhost:3000", true},
		{"long value", "this-is-a-longer-value", false},
		{"random", "a8Kp2xR9mQ4wZ7vN", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSafeValue(tt.value)
			if got != tt.want {
				t.Errorf("isSafeValue(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
