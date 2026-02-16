package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"filippo.io/age/armor"
	"io"
)

// --- Tests for secret share ---

func TestSecretShareCmd_NoArgs(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "share"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing argument, got nil")
	}
}

func TestSecretShareCmd_NoRecipient(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "share", "API_KEY"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no recipient specified, got nil")
	}
	if !contains(err.Error(), "--to or --to-file is required") {
		t.Errorf("expected recipient error, got: %v", err)
	}
}

func TestSecretShareCmd_MutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	// Create a dummy key file.
	keyFile := filepath.Join(dir, "key.pub")
	if err := os.WriteFile(keyFile, []byte("age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p\n"), 0o644); err != nil {
		t.Fatalf("writing key file: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "share", "API_KEY", "--to", "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p", "--to-file", keyFile})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags, got nil")
	}
	if !contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutually exclusive error, got: %v", err)
	}
}

func TestSecretShareCmd_InvalidPublicKey(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "share", "API_KEY", "--to", "not-a-valid-key"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid public key, got nil")
	}
	if !contains(err.Error(), "invalid age public key") {
		t.Errorf("expected invalid key error, got: %v", err)
	}
}

func TestSecretShareCmd_NoConfig(t *testing.T) {
	dir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "share", "API_KEY", "--to", "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no config found, got nil")
	}
	if !contains(err.Error(), "loading config") {
		t.Errorf("expected config loading error, got: %v", err)
	}
}

func TestSecretShareCmd_NoBackends(t *testing.T) {
	dir := t.TempDir()
	content := "project: testproject\n"
	if err := os.WriteFile(filepath.Join(dir, ".envref.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "share", "API_KEY", "--to", "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no backends configured, got nil")
	}
	if !contains(err.Error(), "no backends configured") {
		t.Errorf("expected 'no backends configured' error, got: %v", err)
	}
}

func TestSecretShareCmd_InvalidBackend(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "share", "API_KEY", "--to", "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p", "--backend", "nonexistent"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent backend, got nil")
	}
	if !contains(err.Error(), "nonexistent") {
		t.Errorf("expected error mentioning backend name, got: %v", err)
	}
}

func TestSecretShareCmd_EmptyKey(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "share", "  ", "--to", "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for empty key, got nil")
	}
	if !contains(err.Error(), "key must not be empty") {
		t.Errorf("expected empty key error, got: %v", err)
	}
}

func TestSecretShareCmd_ToFileNotFound(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "share", "API_KEY", "--to-file", "/nonexistent/key.pub"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent key file, got nil")
	}
	if !contains(err.Error(), "reading public key file") {
		t.Errorf("expected file read error, got: %v", err)
	}
}

func TestSecretShareCmd_ToFileEmpty(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	keyFile := filepath.Join(dir, "empty.pub")
	if err := os.WriteFile(keyFile, []byte(""), 0o644); err != nil {
		t.Fatalf("writing key file: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "share", "API_KEY", "--to-file", keyFile})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for empty key file, got nil")
	}
	if !contains(err.Error(), "is empty") {
		t.Errorf("expected empty file error, got: %v", err)
	}
}

// --- Unit tests for encryption helpers ---

func TestEncryptForRecipient(t *testing.T) {
	// Generate a key pair for testing.
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}
	recipient := identity.Recipient()

	plaintext := "super-secret-value-123"
	encrypted, err := encryptForRecipient(plaintext, recipient)
	if err != nil {
		t.Fatalf("encrypting: %v", err)
	}

	// Verify the output is ASCII-armored.
	if !strings.HasPrefix(encrypted, "-----BEGIN AGE ENCRYPTED FILE-----") {
		t.Errorf("expected armor header, got prefix: %q", encrypted[:50])
	}
	if !strings.HasSuffix(strings.TrimSpace(encrypted), "-----END AGE ENCRYPTED FILE-----") {
		t.Error("expected armor footer")
	}

	// Decrypt and verify the plaintext matches.
	armorReader := armor.NewReader(strings.NewReader(encrypted))
	reader, err := age.Decrypt(armorReader, identity)
	if err != nil {
		t.Fatalf("decrypting: %v", err)
	}
	decrypted, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading decrypted: %v", err)
	}
	if string(decrypted) != plaintext {
		t.Errorf("decrypted value: got %q, want %q", string(decrypted), plaintext)
	}
}

func TestEncryptForRecipient_EmptyPlaintext(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}
	recipient := identity.Recipient()

	encrypted, err := encryptForRecipient("", recipient)
	if err != nil {
		t.Fatalf("encrypting empty plaintext: %v", err)
	}

	// Should still be valid armor.
	if !strings.HasPrefix(encrypted, "-----BEGIN AGE ENCRYPTED FILE-----") {
		t.Error("expected armor header for empty plaintext")
	}

	// Decrypt and verify empty.
	armorReader := armor.NewReader(strings.NewReader(encrypted))
	reader, err := age.Decrypt(armorReader, identity)
	if err != nil {
		t.Fatalf("decrypting: %v", err)
	}
	decrypted, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading decrypted: %v", err)
	}
	if string(decrypted) != "" {
		t.Errorf("expected empty decrypted value, got %q", string(decrypted))
	}
}

func TestResolveRecipientKey_Direct(t *testing.T) {
	key, err := resolveRecipientKey("age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p" {
		t.Errorf("got %q", key)
	}
}

func TestResolveRecipientKey_DirectWithWhitespace(t *testing.T) {
	key, err := resolveRecipientKey("  age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p  \n", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p" {
		t.Errorf("got %q", key)
	}
}

func TestResolveRecipientKey_FromFile(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "recipient.pub")
	content := "# teammate's key\nage1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p\n"
	if err := os.WriteFile(keyFile, []byte(content), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	key, err := resolveRecipientKey("", keyFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p" {
		t.Errorf("got %q", key)
	}
}

func TestResolveRecipientKey_NeitherProvided(t *testing.T) {
	_, err := resolveRecipientKey("", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !contains(err.Error(), "--to or --to-file is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveRecipientKey_BothProvided(t *testing.T) {
	_, err := resolveRecipientKey("age1...", "some-file.pub")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !contains(err.Error(), "mutually exclusive") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveRecipientKey_FileOnlyComments(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "comments.pub")
	content := "# just a comment\n# another comment\n"
	if err := os.WriteFile(keyFile, []byte(content), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	_, err := resolveRecipientKey("", keyFile)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !contains(err.Error(), "no public key found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTruncateKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p", "age1ql3z7hjy54pw..."},
		{"short", "short"},
		{"exactly-twenty-chars", "exactly-twenty-chars"},
		{"exactly-twenty-one!xx", "exactly-twenty-o..."},
	}
	for _, tt := range tests {
		got := truncateKey(tt.input)
		if got != tt.want {
			t.Errorf("truncateKey(%q): got %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsNotFound(t *testing.T) {
	if isNotFound(nil) {
		t.Error("nil should not be not-found")
	}
}
