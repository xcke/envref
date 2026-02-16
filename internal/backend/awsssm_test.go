package backend

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// buildAWSMock compiles the mock aws CLI helper into a temporary directory
// and returns the path to the built executable.
func buildAWSMock(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available, skipping aws-ssm tests")
	}

	dir := t.TempDir()
	binName := "aws"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(dir, binName)

	src := filepath.Join("testdata", "aws_mock.go")
	cmd := exec.Command("go", "build", "-o", binPath, src)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build aws mock: %v", err)
	}
	return binPath
}

func TestAWSSSMBackend_Interface(t *testing.T) {
	var _ Backend = &AWSSSMBackend{}
}

func TestAWSSSMBackend_Name(t *testing.T) {
	b := NewAWSSSMBackend("/envref")
	if b.Name() != "aws-ssm" {
		t.Fatalf("Name(): got %q, want %q", b.Name(), "aws-ssm")
	}
}

func TestAWSSSMBackend_SetGetDeleteList(t *testing.T) {
	awsPath := buildAWSMock(t)
	b := NewAWSSSMBackend("/test", WithAWSSSMCommand(awsPath))

	// List should be empty initially.
	keys, err := b.List()
	if err != nil {
		t.Fatalf("List() initial: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("List() initial: got %v, want empty", keys)
	}

	// Set a key.
	if err := b.Set("api_key", "secret123"); err != nil {
		t.Fatalf("Set(api_key): %v", err)
	}

	// Get the key.
	val, err := b.Get("api_key")
	if err != nil {
		t.Fatalf("Get(api_key): %v", err)
	}
	if val != "secret123" {
		t.Fatalf("Get(api_key): got %q, want %q", val, "secret123")
	}

	// Set another key.
	if err := b.Set("db_pass", "password456"); err != nil {
		t.Fatalf("Set(db_pass): %v", err)
	}

	// Update existing key (overwrite).
	if err := b.Set("api_key", "updated_secret"); err != nil {
		t.Fatalf("Set(api_key) update: %v", err)
	}

	// Verify update.
	val, err = b.Get("api_key")
	if err != nil {
		t.Fatalf("Get(api_key) after update: %v", err)
	}
	if val != "updated_secret" {
		t.Fatalf("Get(api_key) after update: got %q, want %q", val, "updated_secret")
	}

	// List should return both keys.
	keys, err = b.List()
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("List(): got %d keys, want 2", len(keys))
	}

	// Delete.
	if err := b.Delete("api_key"); err != nil {
		t.Fatalf("Delete(api_key): %v", err)
	}

	// Get after delete should return ErrNotFound.
	_, err = b.Get("api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(deleted): got %v, want ErrNotFound", err)
	}

	// List should have one key left.
	keys, err = b.List()
	if err != nil {
		t.Fatalf("List() after delete: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("List() after delete: got %d keys, want 1", len(keys))
	}
	if keys[0] != "db_pass" {
		t.Fatalf("List() after delete: got %q, want %q", keys[0], "db_pass")
	}
}

func TestAWSSSMBackend_GetNotFound(t *testing.T) {
	awsPath := buildAWSMock(t)
	b := NewAWSSSMBackend("/test", WithAWSSSMCommand(awsPath))

	_, err := b.Get("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(nonexistent): got %v, want ErrNotFound", err)
	}
}

func TestAWSSSMBackend_DeleteNotFound(t *testing.T) {
	awsPath := buildAWSMock(t)
	b := NewAWSSSMBackend("/test", WithAWSSSMCommand(awsPath))

	err := b.Delete("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(nonexistent): got %v, want ErrNotFound", err)
	}
}

func TestAWSSSMBackend_InvalidCommand(t *testing.T) {
	b := NewAWSSSMBackend("/test", WithAWSSSMCommand("/nonexistent/aws"))

	_, err := b.Get("key")
	if err == nil {
		t.Fatal("Get with invalid command: expected error, got nil")
	}
}

func TestAWSSSMBackend_Options(t *testing.T) {
	b := NewAWSSSMBackend("/myapp/prod",
		WithAWSSSMRegion("us-west-2"),
		WithAWSSSMProfile("production"),
		WithAWSSSMCommand("/usr/local/bin/aws"),
	)

	if b.prefix != "/myapp/prod" {
		t.Fatalf("prefix: got %q, want %q", b.prefix, "/myapp/prod")
	}
	if b.region != "us-west-2" {
		t.Fatalf("region: got %q, want %q", b.region, "us-west-2")
	}
	if b.profile != "production" {
		t.Fatalf("profile: got %q, want %q", b.profile, "production")
	}
	if b.command != "/usr/local/bin/aws" {
		t.Fatalf("command: got %q, want %q", b.command, "/usr/local/bin/aws")
	}
}

func TestAWSSSMBackend_ParamName(t *testing.T) {
	b := NewAWSSSMBackend("/myapp/prod")

	got := b.paramName("api_key")
	want := "/myapp/prod/api_key"
	if got != want {
		t.Fatalf("paramName(api_key): got %q, want %q", got, want)
	}
}

func TestIsAWSNotFoundErr(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"An error occurred (ParameterNotFound) when calling the GetParameter operation", true},
		{"parameter not found", true},
		{"parameter /test/key does not exist", true},
		{"An error occurred (ParameterNotFoundException)", true},
		{"access denied", false},
		{"throttling exception", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isAWSNotFoundErr(errors.New(tt.msg))
		if got != tt.want {
			t.Errorf("isAWSNotFoundErr(%q): got %v, want %v", tt.msg, got, tt.want)
		}
	}
}
