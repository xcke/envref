package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"filippo.io/age/armor"
	"io"
)

// --- Tests for sync push ---

func TestSyncPushCmd_NoRecipients(t *testing.T) {
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
	root.SetArgs([]string{"sync", "push"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for no recipients, got nil")
	}
	if !contains(err.Error(), "at least one recipient") {
		t.Errorf("expected recipient error, got: %v", err)
	}
}

func TestSyncPushCmd_NoConfig(t *testing.T) {
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
	root.SetArgs([]string{"sync", "push", "--to", "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for no config, got nil")
	}
	if !contains(err.Error(), "loading config") {
		t.Errorf("expected config error, got: %v", err)
	}
}

func TestSyncPushCmd_NoBackends(t *testing.T) {
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
	root.SetArgs([]string{"sync", "push", "--to", "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for no backends, got nil")
	}
	if !contains(err.Error(), "no backends configured") {
		t.Errorf("expected backends error, got: %v", err)
	}
}

func TestSyncPushCmd_InvalidBackend(t *testing.T) {
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
	root.SetArgs([]string{"sync", "push", "--to", "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p", "--backend", "nonexistent"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid backend, got nil")
	}
	if !contains(err.Error(), "nonexistent") {
		t.Errorf("expected error mentioning backend name, got: %v", err)
	}
}

func TestSyncPushCmd_InvalidPublicKey(t *testing.T) {
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
	root.SetArgs([]string{"sync", "push", "--to", "not-a-valid-key"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid key, got nil")
	}
	if !contains(err.Error(), "invalid age public key") {
		t.Errorf("expected invalid key error, got: %v", err)
	}
}

// --- Tests for sync pull ---

func TestSyncPullCmd_NoIdentity(t *testing.T) {
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

	// Clear the env var in case it's set.
	t.Setenv("AGE_IDENTITY", "")

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"sync", "pull"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for no identity, got nil")
	}
	if !contains(err.Error(), "identity file is required") {
		t.Errorf("expected identity error, got: %v", err)
	}
}

func TestSyncPullCmd_NoConfig(t *testing.T) {
	dir := t.TempDir()

	// Create a dummy identity file.
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}
	keyFile := filepath.Join(dir, "key.txt")
	if err := os.WriteFile(keyFile, []byte(identity.String()+"\n"), 0o600); err != nil {
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
	root.SetArgs([]string{"sync", "pull", "--identity", keyFile})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for no config, got nil")
	}
	if !contains(err.Error(), "loading config") {
		t.Errorf("expected config error, got: %v", err)
	}
}

func TestSyncPullCmd_NoBackends(t *testing.T) {
	dir := t.TempDir()
	content := "project: testproject\n"
	if err := os.WriteFile(filepath.Join(dir, ".envref.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}
	keyFile := filepath.Join(dir, "key.txt")
	if err := os.WriteFile(keyFile, []byte(identity.String()+"\n"), 0o600); err != nil {
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
	root.SetArgs([]string{"sync", "pull", "--identity", keyFile})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for no backends, got nil")
	}
	if !contains(err.Error(), "no backends configured") {
		t.Errorf("expected backends error, got: %v", err)
	}
}

func TestSyncPullCmd_MissingFile(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}
	keyFile := filepath.Join(dir, "key.txt")
	if err := os.WriteFile(keyFile, []byte(identity.String()+"\n"), 0o600); err != nil {
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
	root.SetArgs([]string{"sync", "pull", "--identity", keyFile})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for missing sync file, got nil")
	}
	if !contains(err.Error(), "reading sync file") {
		t.Errorf("expected file read error, got: %v", err)
	}
}

func TestSyncPullCmd_InvalidIdentityFile(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	// Write an invalid identity file.
	keyFile := filepath.Join(dir, "bad-key.txt")
	if err := os.WriteFile(keyFile, []byte("not-a-valid-identity\n"), 0o600); err != nil {
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
	root.SetArgs([]string{"sync", "pull", "--identity", keyFile})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid identity, got nil")
	}
	if !contains(err.Error(), "parsing identity file") {
		t.Errorf("expected identity parse error, got: %v", err)
	}
}

func TestSyncPullCmd_IdentityFromEnvVar(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}
	keyFile := filepath.Join(dir, "key.txt")
	if err := os.WriteFile(keyFile, []byte(identity.String()+"\n"), 0o600); err != nil {
		t.Fatalf("writing key file: %v", err)
	}

	// Set the env var.
	t.Setenv("AGE_IDENTITY", keyFile)

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
	// No --identity flag; should pick up from env var.
	root.SetArgs([]string{"sync", "pull"})

	err = root.Execute()
	// Should fail because the sync file doesn't exist, not because of identity.
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !contains(err.Error(), "reading sync file") {
		t.Errorf("expected file read error (identity resolved from env), got: %v", err)
	}
}

// --- Unit tests for encryption/decryption helpers ---

func TestCollectRecipients_DirectKeys(t *testing.T) {
	identity1, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}
	identity2, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}

	recipients, err := collectRecipients(
		[]string{identity1.Recipient().String(), identity2.Recipient().String()},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recipients) != 2 {
		t.Errorf("expected 2 recipients, got %d", len(recipients))
	}
}

func TestCollectRecipients_FromFile(t *testing.T) {
	identity1, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}
	identity2, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}

	dir := t.TempDir()
	keyFile := filepath.Join(dir, "keys.txt")
	content := "# Team keys\n" +
		identity1.Recipient().String() + "\n" +
		"# Alice\n" +
		identity2.Recipient().String() + "\n"
	if err := os.WriteFile(keyFile, []byte(content), 0o644); err != nil {
		t.Fatalf("writing key file: %v", err)
	}

	recipients, err := collectRecipients(nil, []string{keyFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recipients) != 2 {
		t.Errorf("expected 2 recipients, got %d", len(recipients))
	}
}

func TestCollectRecipients_InvalidKey(t *testing.T) {
	_, err := collectRecipients([]string{"not-a-valid-key"}, nil)
	if err == nil {
		t.Fatal("expected error for invalid key, got nil")
	}
	if !contains(err.Error(), "invalid age public key") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCollectRecipients_Empty(t *testing.T) {
	recipients, err := collectRecipients(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recipients) != 0 {
		t.Errorf("expected 0 recipients, got %d", len(recipients))
	}
}

func TestCollectRecipients_EmptyKeys(t *testing.T) {
	recipients, err := collectRecipients([]string{"", "  "}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recipients) != 0 {
		t.Errorf("expected 0 recipients for empty keys, got %d", len(recipients))
	}
}

func TestCollectRecipients_MissingFile(t *testing.T) {
	_, err := collectRecipients(nil, []string{"/nonexistent/keys.txt"})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !contains(err.Error(), "reading key file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEncryptForRecipients_SingleRecipient(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}

	plaintext := `{"API_KEY":"secret123","DB_PASS":"hunter2"}`
	encrypted, err := encryptForRecipients(plaintext, []*age.X25519Recipient{identity.Recipient()})
	if err != nil {
		t.Fatalf("encrypting: %v", err)
	}

	// Verify armor format.
	if !strings.HasPrefix(encrypted, "-----BEGIN AGE ENCRYPTED FILE-----") {
		t.Error("expected armor header")
	}

	// Decrypt and verify.
	decrypted, err := decryptWithIdentities(encrypted, []age.Identity{identity})
	if err != nil {
		t.Fatalf("decrypting: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("decrypted value: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptForRecipients_MultipleRecipients(t *testing.T) {
	identity1, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity 1: %v", err)
	}
	identity2, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity 2: %v", err)
	}

	plaintext := `{"SECRET":"value"}`
	encrypted, err := encryptForRecipients(plaintext, []*age.X25519Recipient{
		identity1.Recipient(),
		identity2.Recipient(),
	})
	if err != nil {
		t.Fatalf("encrypting: %v", err)
	}

	// Both identities should be able to decrypt.
	for i, id := range []age.Identity{identity1, identity2} {
		decrypted, err := decryptWithIdentities(encrypted, []age.Identity{id})
		if err != nil {
			t.Fatalf("identity %d: decrypting: %v", i+1, err)
		}
		if decrypted != plaintext {
			t.Errorf("identity %d: decrypted value: got %q, want %q", i+1, decrypted, plaintext)
		}
	}
}

func TestDecryptWithIdentities_WrongKey(t *testing.T) {
	// Encrypt for identity1.
	identity1, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity 1: %v", err)
	}
	identity2, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity 2: %v", err)
	}

	encrypted, err := encryptForRecipients("secret", []*age.X25519Recipient{identity1.Recipient()})
	if err != nil {
		t.Fatalf("encrypting: %v", err)
	}

	// Try to decrypt with identity2 — should fail.
	_, err = decryptWithIdentities(encrypted, []age.Identity{identity2})
	if err == nil {
		t.Fatal("expected decryption error with wrong key, got nil")
	}
}

func TestParseIdentityFile(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}

	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key.txt")
	content := "# created: 2024-01-01\n# public key: " + identity.Recipient().String() + "\n" + identity.String() + "\n"
	if err := os.WriteFile(keyFile, []byte(content), 0o600); err != nil {
		t.Fatalf("writing key file: %v", err)
	}

	identities, err := parseIdentityFile(keyFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(identities) != 1 {
		t.Errorf("expected 1 identity, got %d", len(identities))
	}
}

func TestParseIdentityFile_NotFound(t *testing.T) {
	_, err := parseIdentityFile("/nonexistent/key.txt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !contains(err.Error(), "opening identity file") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Round-trip test: encrypt JSON secrets → decrypt → verify ---

func TestSyncRoundTrip(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}

	// Simulate push: serialize secrets to JSON and encrypt.
	secrets := map[string]string{
		"API_KEY":  "sk-abc123",
		"DB_PASS":  "hunter2",
		"EMPTY":    "",
		"SPECIAL":  "value with spaces & special=chars!",
		"MULTILINE": "line1\nline2\nline3",
	}

	jsonData, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		t.Fatalf("marshaling: %v", err)
	}

	encrypted, err := encryptForRecipients(string(jsonData), []*age.X25519Recipient{identity.Recipient()})
	if err != nil {
		t.Fatalf("encrypting: %v", err)
	}

	// Verify it's ASCII armored.
	if !strings.HasPrefix(encrypted, "-----BEGIN AGE ENCRYPTED FILE-----") {
		t.Error("expected armor header")
	}

	// Simulate pull: decrypt and parse JSON.
	armorReader := armor.NewReader(strings.NewReader(encrypted))
	reader, err := age.Decrypt(armorReader, identity)
	if err != nil {
		t.Fatalf("decrypting: %v", err)
	}
	plaintext, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading decrypted: %v", err)
	}

	var recovered map[string]string
	if err := json.Unmarshal(plaintext, &recovered); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}

	// Verify all secrets match.
	for key, want := range secrets {
		got, ok := recovered[key]
		if !ok {
			t.Errorf("missing key %q in recovered secrets", key)
			continue
		}
		if got != want {
			t.Errorf("key %q: got %q, want %q", key, got, want)
		}
	}
	if len(recovered) != len(secrets) {
		t.Errorf("recovered %d secrets, want %d", len(recovered), len(secrets))
	}
}
