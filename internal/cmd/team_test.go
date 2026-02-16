package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
	"github.com/xcke/envref/internal/config"
)

// --- Tests for team list ---

func TestTeamListCmd_NoTeam(t *testing.T) {
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

	stdout := new(bytes.Buffer)
	root := NewRootCmd()
	root.SetOut(stdout)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"team", "list"})

	err = root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(stdout.String(), "no team members configured") {
		t.Errorf("expected 'no team members configured', got: %q", stdout.String())
	}
}

func TestTeamListCmd_WithTeam(t *testing.T) {
	dir := t.TempDir()
	identity1, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}
	identity2, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}

	cfgContent := "project: testproject\nbackends:\n  - name: keychain\n    type: keychain\nteam:\n  - name: alice\n    public_key: " +
		identity1.Recipient().String() + "\n  - name: bob\n    public_key: " +
		identity2.Recipient().String() + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".envref.yaml"), []byte(cfgContent), 0o644); err != nil {
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

	stdout := new(bytes.Buffer)
	root := NewRootCmd()
	root.SetOut(stdout)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"team", "list"})

	err = root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !contains(out, "alice") {
		t.Errorf("expected 'alice' in output, got: %q", out)
	}
	if !contains(out, "bob") {
		t.Errorf("expected 'bob' in output, got: %q", out)
	}
	if !contains(out, identity1.Recipient().String()) {
		t.Errorf("expected alice's key in output, got: %q", out)
	}
}

func TestTeamListCmd_NoConfig(t *testing.T) {
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
	root.SetArgs([]string{"team", "list"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for no config, got nil")
	}
	if !contains(err.Error(), "loading config") {
		t.Errorf("expected config error, got: %v", err)
	}
}

// --- Tests for team add ---

func TestTeamAddCmd_Success(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
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

	stdout := new(bytes.Buffer)
	root := NewRootCmd()
	root.SetOut(stdout)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"team", "add", "alice", identity.Recipient().String()})

	err = root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(stdout.String(), "added team member") {
		t.Errorf("expected success message, got: %q", stdout.String())
	}

	// Verify the config was updated.
	cfg, loadErr := config.LoadFile(filepath.Join(dir, ".envref.yaml"))
	if loadErr != nil {
		t.Fatalf("loading config: %v", loadErr)
	}
	if len(cfg.Team) != 1 {
		t.Fatalf("expected 1 team member, got %d", len(cfg.Team))
	}
	if cfg.Team[0].Name != "alice" {
		t.Errorf("team member name = %q, want %q", cfg.Team[0].Name, "alice")
	}
	if cfg.Team[0].PublicKey != identity.Recipient().String() {
		t.Errorf("team member key = %q, want %q", cfg.Team[0].PublicKey, identity.Recipient().String())
	}
}

func TestTeamAddCmd_DuplicateName(t *testing.T) {
	dir := t.TempDir()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}

	cfgContent := "project: testproject\nbackends:\n  - name: keychain\n    type: keychain\nteam:\n  - name: alice\n    public_key: " +
		identity.Recipient().String() + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".envref.yaml"), []byte(cfgContent), 0o644); err != nil {
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

	identity2, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"team", "add", "alice", identity2.Recipient().String()})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}
	if !contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestTeamAddCmd_MissingArgs(t *testing.T) {
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
	root.SetArgs([]string{"team", "add", "alice"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for missing args, got nil")
	}
}

// --- Tests for team remove ---

func TestTeamRemoveCmd_Success(t *testing.T) {
	dir := t.TempDir()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating identity: %v", err)
	}

	cfgContent := "project: testproject\nbackends:\n  - name: keychain\n    type: keychain\nteam:\n  - name: alice\n    public_key: " +
		identity.Recipient().String() + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".envref.yaml"), []byte(cfgContent), 0o644); err != nil {
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

	stdout := new(bytes.Buffer)
	root := NewRootCmd()
	root.SetOut(stdout)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"team", "remove", "alice"})

	err = root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(stdout.String(), "removed team member") {
		t.Errorf("expected success message, got: %q", stdout.String())
	}

	// Verify the config was updated.
	cfg, loadErr := config.LoadFile(filepath.Join(dir, ".envref.yaml"))
	if loadErr != nil {
		t.Fatalf("loading config: %v", loadErr)
	}
	if len(cfg.Team) != 0 {
		t.Fatalf("expected 0 team members, got %d", len(cfg.Team))
	}
}

func TestTeamRemoveCmd_NotFound(t *testing.T) {
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
	root.SetArgs([]string{"team", "remove", "nobody"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for missing member, got nil")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestTeamRemoveCmd_MissingArgs(t *testing.T) {
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
	root.SetArgs([]string{"team", "remove"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for missing args, got nil")
	}
}

// --- Tests for sync push --to-team ---

func TestSyncPushCmd_ToTeamNoMembers(t *testing.T) {
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
	root.SetArgs([]string{"sync", "push", "--to-team"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for no team members, got nil")
	}
	if !contains(err.Error(), "no team members configured") {
		t.Errorf("expected 'no team members configured' error, got: %v", err)
	}
}
