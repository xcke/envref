//go:build integration

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/xcke/envref/internal/config"
)

// =============================================================================
// Integration tests for direnv integration (end-to-end shell tests).
//
// These tests build the actual envref binary and test the full direnv workflow:
//   - eval "$(envref resolve --direnv)" injects vars into the shell
//   - init --direnv generates a correct .envrc
//   - profile-aware direnv output works end-to-end
//   - shell quoting is safe for eval
//   - strict mode produces no output on failure
//   - if direnv is installed, test the actual direnv eval flow
//
// Run with: go test -tags=integration -v ./internal/cmd/... -run TestDirenvIntegration
// =============================================================================

var (
	direnvBinaryOnce sync.Once
	direnvBinaryPath string
	direnvBinaryErr  error
)

// ensureTestBinary compiles the envref binary once and returns its path.
// The binary lives in a temp directory that persists for the test process lifetime.
func ensureTestBinary(t *testing.T) string {
	t.Helper()
	direnvBinaryOnce.Do(func() {
		tmpDir, err := os.MkdirTemp("", "envref-direnv-test-*")
		if err != nil {
			direnvBinaryErr = fmt.Errorf("creating temp dir: %w", err)
			return
		}

		binaryName := "envref"
		if runtime.GOOS == "windows" {
			binaryName = "envref.exe"
		}
		binPath := filepath.Join(tmpDir, binaryName)

		modRoot := findModRoot()
		cmd := exec.Command("go", "build", "-o", binPath, "./cmd/envref")
		cmd.Dir = modRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			direnvBinaryErr = fmt.Errorf("building envref: %v\n%s", err, out)
			return
		}
		direnvBinaryPath = binPath
	})
	if direnvBinaryErr != nil {
		t.Fatalf("building test binary: %v", direnvBinaryErr)
	}
	return direnvBinaryPath
}

// findModRoot returns the module root (two levels up from internal/cmd/).
func findModRoot() string {
	dir, err := filepath.Abs("../..")
	if err != nil {
		return "."
	}
	return dir
}

// mkDirenvProject creates a temp directory with .envref.yaml, .env, and
// optionally .env.local files. Returns the directory path.
func mkDirenvProject(t *testing.T, project, envContent, localContent string) string {
	t.Helper()
	dir := t.TempDir()

	cfgContent := "project: " + project + "\nenv_file: .env\nlocal_file: .env.local\n"
	writeFile(t, dir, config.FullFileName, cfgContent)
	if envContent != "" {
		writeFile(t, dir, ".env", envContent)
	}
	if localContent != "" {
		writeFile(t, dir, ".env.local", localContent)
	}
	return dir
}

// writeFile is a helper to write a file in a directory.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

// shellRun executes a shell script in the given directory with the envref
// binary on PATH. Returns stdout, stderr, and any error.
func shellRun(t *testing.T, dir, script, envrefBin string) (string, string, error) {
	t.Helper()

	binDir := filepath.Dir(envrefBin)
	pathEnv := binDir + ":" + os.Getenv("PATH")

	cmd := exec.Command("/bin/sh", "-c", script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+pathEnv)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// --- Init + Direnv Tests ---

func TestDirenvIntegration_InitDirenv_CreatesEnvrc(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := t.TempDir()

	stdout, stderr, err := shellRun(t, dir, bin+" init --project testapp --direnv", bin)
	if err != nil {
		t.Fatalf("init --direnv failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	envrcData, err := os.ReadFile(filepath.Join(dir, ".envrc"))
	if err != nil {
		t.Fatalf("reading .envrc: %v", err)
	}

	envrcStr := string(envrcData)
	if !strings.Contains(envrcStr, "envref resolve --direnv") {
		t.Errorf(".envrc should contain 'envref resolve --direnv', got:\n%s", envrcStr)
	}
	if !strings.Contains(envrcStr, "2>/dev/null") {
		t.Errorf(".envrc should redirect stderr to /dev/null, got:\n%s", envrcStr)
	}
	if !strings.Contains(envrcStr, "|| true") {
		t.Errorf(".envrc should have '|| true' fallback, got:\n%s", envrcStr)
	}
}

func TestDirenvIntegration_InitDirenv_EnvrcIsEvalSafe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := t.TempDir()

	_, _, err := shellRun(t, dir, bin+" init --project testapp --direnv", bin)
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	// Resolve via the binary to verify the output is valid shell.
	stdout, _, err := shellRun(t, dir, bin+" resolve --direnv", bin)
	if err != nil {
		t.Fatalf("resolve --direnv: %v", err)
	}

	if !strings.Contains(stdout, "export APP_NAME=myapp") {
		t.Errorf("expected 'export APP_NAME=myapp', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "export APP_ENV=development") {
		t.Errorf("expected 'export APP_ENV=development', got:\n%s", stdout)
	}
}

func TestDirenvIntegration_InitWithoutDirenv_NoEnvrc(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := t.TempDir()

	_, _, err := shellRun(t, dir, bin+" init --project testapp", bin)
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".envrc")); err == nil {
		t.Error("init without --direnv should not create .envrc")
	}
}

// --- Shell Eval Tests ---

func TestDirenvIntegration_EvalDirenvOutput_InjectsVars(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := mkDirenvProject(t, "testproject", "HOST=myhost\nPORT=9090\n", "")

	script := fmt.Sprintf(`eval "$(%s resolve --direnv)" && echo "HOST=$HOST PORT=$PORT"`, bin)
	stdout, _, err := shellRun(t, dir, script, bin)
	if err != nil {
		t.Fatalf("eval + echo: %v", err)
	}

	got := strings.TrimSpace(stdout)
	if got != "HOST=myhost PORT=9090" {
		t.Errorf("expected 'HOST=myhost PORT=9090', got %q", got)
	}
}

func TestDirenvIntegration_EvalDirenvOutput_LocalOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := mkDirenvProject(t, "testproject", "APP=production\n", "APP=local_dev\n")

	script := fmt.Sprintf(`eval "$(%s resolve --direnv)" && echo "$APP"`, bin)
	stdout, _, err := shellRun(t, dir, script, bin)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	got := strings.TrimSpace(stdout)
	if got != "local_dev" {
		t.Errorf("expected .env.local override 'local_dev', got %q", got)
	}
}

func TestDirenvIntegration_EvalDirenvOutput_Interpolation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := mkDirenvProject(t, "testproject",
		"DB_HOST=localhost\nDB_PORT=5432\nDB_URL=postgres://${DB_HOST}:${DB_PORT}/mydb\n", "")

	script := fmt.Sprintf(`eval "$(%s resolve --direnv)" && echo "$DB_URL"`, bin)
	stdout, _, err := shellRun(t, dir, script, bin)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	got := strings.TrimSpace(stdout)
	expected := "postgres://localhost:5432/mydb"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestDirenvIntegration_EvalDirenvOutput_QuotedValues(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := mkDirenvProject(t, "testproject",
		"SIMPLE=hello\nSPACES=hello world\nQUOTED=\"double quoted\"\nEMPTY=\n", "")

	script := fmt.Sprintf(
		`eval "$(%s resolve --direnv)" && echo "SIMPLE=$SIMPLE" && echo "SPACES=$SPACES" && echo "QUOTED=$QUOTED" && echo "EMPTY=[$EMPTY]"`,
		bin)
	stdout, _, err := shellRun(t, dir, script, bin)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	expected := []string{
		"SIMPLE=hello",
		"SPACES=hello world",
		"QUOTED=double quoted",
		"EMPTY=[]",
	}

	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(expected), len(lines), stdout)
	}
	for i, want := range expected {
		if lines[i] != want {
			t.Errorf("line %d: got %q, want %q", i, lines[i], want)
		}
	}
}

func TestDirenvIntegration_EvalDirenvOutput_SpecialChars(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)

	// Values with shell-sensitive characters that must survive eval.
	// Backticks must be quoted in .env to avoid shell interpretation by the parser.
	envContent := "DOLLAR=price$100\n" +
		"BACKTICK='`echo hello`'\n" +
		"BANG=hello!world\n" +
		"SEMICOLON=a;b;c\n" +
		"PIPE=a|b\n" +
		"AMPERSAND=a&b\n" +
		"PARENS=(hello)\n" +
		"HASH=color#red\n"

	dir := mkDirenvProject(t, "testproject", envContent, "")

	tests := []struct {
		varName string
		want    string
	}{
		{"DOLLAR", "price$100"},
		{"BACKTICK", "`echo hello`"},
		{"BANG", "hello!world"},
		{"SEMICOLON", "a;b;c"},
		{"PIPE", "a|b"},
		{"AMPERSAND", "a&b"},
		{"PARENS", "(hello)"},
		{"HASH", "color#red"},
	}

	for _, tt := range tests {
		t.Run(tt.varName, func(t *testing.T) {
			outFile := filepath.Join(dir, "out_"+tt.varName+".txt")
			// Use printf to write the var value to a file (avoids echo interpretation issues).
			script := fmt.Sprintf(
				`eval "$(%s resolve --direnv)" && printf '%%s' "$%s" > '%s'`,
				bin, tt.varName, outFile)

			_, stderr, err := shellRun(t, dir, script, bin)
			if err != nil {
				t.Fatalf("eval failed: %v\nstderr: %s", err, stderr)
			}

			data, err := os.ReadFile(outFile)
			if err != nil {
				t.Fatalf("reading output: %v", err)
			}
			if string(data) != tt.want {
				t.Errorf("got %q, want %q", string(data), tt.want)
			}
		})
	}
}

func TestDirenvIntegration_EvalDirenvOutput_MultilineValue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := mkDirenvProject(t, "testproject",
		"CERT=\"line1\nline2\nline3\"\n", "")

	outFile := filepath.Join(dir, "out.txt")
	script := fmt.Sprintf(
		`eval "$(%s resolve --direnv)" && printf '%%s' "$CERT" > '%s'`,
		bin, outFile)
	_, stderr, err := shellRun(t, dir, script, bin)
	if err != nil {
		t.Fatalf("eval: %v\nstderr: %s", err, stderr)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(data) != "line1\nline2\nline3" {
		t.Errorf("multiline value: got %q, want %q", string(data), "line1\nline2\nline3")
	}
}

func TestDirenvIntegration_EvalDirenvOutput_SingleQuotesInValue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := mkDirenvProject(t, "testproject",
		"MSG=\"it's a test\"\n", "")

	outFile := filepath.Join(dir, "out.txt")
	script := fmt.Sprintf(
		`eval "$(%s resolve --direnv)" && printf '%%s' "$MSG" > '%s'`,
		bin, outFile)
	_, stderr, err := shellRun(t, dir, script, bin)
	if err != nil {
		t.Fatalf("eval: %v\nstderr: %s", err, stderr)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(data) != "it's a test" {
		t.Errorf("expected \"it's a test\", got %q", string(data))
	}
}

// --- Profile + Direnv Tests ---

func TestDirenvIntegration_EvalDirenvOutput_WithProfile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := t.TempDir()

	cfgContent := "project: testproject\nprofiles:\n  staging:\n    env_file: .env.staging\n"
	writeFile(t, dir, config.FullFileName, cfgContent)
	writeFile(t, dir, ".env", "HOST=prod.example.com\nPORT=443\n")
	writeFile(t, dir, ".env.staging", "HOST=staging.example.com\nPORT=8443\n")

	script := fmt.Sprintf(
		`eval "$(%s resolve --direnv --profile staging)" && echo "HOST=$HOST PORT=$PORT"`, bin)
	stdout, _, err := shellRun(t, dir, script, bin)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	got := strings.TrimSpace(stdout)
	if got != "HOST=staging.example.com PORT=8443" {
		t.Errorf("expected staging values, got %q", got)
	}
}

func TestDirenvIntegration_EvalDirenvOutput_ActiveProfile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := t.TempDir()

	cfgContent := "project: testproject\nactive_profile: production\nprofiles:\n  production:\n    env_file: .env.production\n"
	writeFile(t, dir, config.FullFileName, cfgContent)
	writeFile(t, dir, ".env", "DB=default-db\n")
	writeFile(t, dir, ".env.production", "DB=prod-db\n")

	// No --profile flag; should pick up active_profile from config.
	script := fmt.Sprintf(`eval "$(%s resolve --direnv)" && echo "$DB"`, bin)
	stdout, _, err := shellRun(t, dir, script, bin)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	got := strings.TrimSpace(stdout)
	if got != "prod-db" {
		t.Errorf("expected 'prod-db' from active_profile, got %q", got)
	}
}

func TestDirenvIntegration_EvalDirenvOutput_ProfileWithLocalOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := t.TempDir()

	cfgContent := "project: testproject\nprofiles:\n  staging:\n    env_file: .env.staging\n"
	writeFile(t, dir, config.FullFileName, cfgContent)
	writeFile(t, dir, ".env", "HOST=base\n")
	writeFile(t, dir, ".env.staging", "HOST=staging\n")
	writeFile(t, dir, ".env.local", "HOST=local-override\n")

	script := fmt.Sprintf(`eval "$(%s resolve --direnv --profile staging)" && echo "$HOST"`, bin)
	stdout, _, err := shellRun(t, dir, script, bin)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	got := strings.TrimSpace(stdout)
	if got != "local-override" {
		t.Errorf("expected .env.local to win over profile, got %q", got)
	}
}

// --- Strict Mode Tests ---

func TestDirenvIntegration_StrictMode_NoRefsSucceeds(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := mkDirenvProject(t, "testproject", "KEY=value\n", "")

	script := fmt.Sprintf(`eval "$(%s resolve --direnv --strict)" && echo "KEY=$KEY"`, bin)
	stdout, _, err := shellRun(t, dir, script, bin)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	got := strings.TrimSpace(stdout)
	if got != "KEY=value" {
		t.Errorf("expected 'KEY=value', got %q", got)
	}
}

func TestDirenvIntegration_StrictMode_FailedRefs_NoOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := t.TempDir()

	cfgContent := "project: testproject\nbackends:\n  - name: keychain\n    type: keychain\n"
	writeFile(t, dir, config.FullFileName, cfgContent)
	writeFile(t, dir, ".env", "SAFE=ok\nSECRET=ref://keychain/missing_key\n")

	// With --strict, resolve should fail and produce no stdout.
	stdout, _, err := shellRun(t, dir, bin+" resolve --direnv --strict", bin)
	if err == nil {
		t.Log("Warning: expected non-zero exit but got success (keychain may have the key)")
	}

	// Strict mode: no partial exports on failure.
	if stdout != "" && strings.Contains(stdout, "export") {
		t.Errorf("strict mode should produce no export output on failure, got:\n%s", stdout)
	}
}

// --- Full Workflow Tests ---

func TestDirenvIntegration_FullWorkflow_InitSetResolveEval(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := t.TempDir()

	// Step 1: Init the project with direnv.
	_, stderr, err := shellRun(t, dir, bin+" init --project myapp --direnv", bin)
	if err != nil {
		t.Fatalf("init: %v\nstderr: %s", err, stderr)
	}

	// Step 2: Set additional values.
	envPath := filepath.Join(dir, ".env")
	_, stderr, err = shellRun(t, dir, bin+" set CUSTOM_VAR=custom_value --file "+envPath, bin)
	if err != nil {
		t.Fatalf("set: %v\nstderr: %s", err, stderr)
	}

	// Step 3: Eval the direnv output and verify all vars are available.
	script := fmt.Sprintf(
		`eval "$(%s resolve --direnv)" && echo "APP=$APP_NAME CUSTOM=$CUSTOM_VAR"`, bin)
	stdout, _, err := shellRun(t, dir, script, bin)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	got := strings.TrimSpace(stdout)
	if !strings.Contains(got, "APP=myapp") {
		t.Errorf("expected APP=myapp in output, got %q", got)
	}
	if !strings.Contains(got, "CUSTOM=custom_value") {
		t.Errorf("expected CUSTOM=custom_value in output, got %q", got)
	}
}

func TestDirenvIntegration_FullWorkflow_EnvrcEval(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := t.TempDir()

	// Init with direnv to create .envrc.
	_, stderr, err := shellRun(t, dir, bin+" init --project evaltest --direnv", bin)
	if err != nil {
		t.Fatalf("init: %v\nstderr: %s", err, stderr)
	}

	// Source the generated .envrc in a shell. The .envrc calls envref
	// which must be on PATH. Use ./ prefix for POSIX sh compatibility.
	outFile := filepath.Join(dir, "out.txt")
	script := fmt.Sprintf(`. ./.envrc && printf '%%s' "$APP_NAME" > '%s'`, outFile)
	_, stderr, err = shellRun(t, dir, script, bin)
	if err != nil {
		t.Fatalf("sourcing .envrc: %v\nstderr: %s", err, stderr)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(data) != "myapp" {
		t.Errorf("expected 'myapp' from .envrc eval, got %q", string(data))
	}
}

// --- Direnv Binary Tests (only run if direnv is installed) ---

func TestDirenvIntegration_RealDirenv_EvalFlow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}

	direnvBin, err := exec.LookPath("direnv")
	if err != nil {
		t.Skip("skipping: direnv not installed")
	}
	t.Logf("using direnv at: %s", direnvBin)

	bin := ensureTestBinary(t)
	dir := t.TempDir()

	writeFile(t, dir, config.FullFileName, "project: testproject\nenv_file: .env\nlocal_file: .env.local\n")
	writeFile(t, dir, ".env", "DIRENV_TEST_KEY=direnv_works\n")
	writeFile(t, dir, ".envrc", `eval "$(envref resolve --direnv 2>/dev/null)" || true`)

	binDir := filepath.Dir(bin)
	pathEnv := binDir + ":" + os.Getenv("PATH")

	allowCmd := exec.Command(direnvBin, "allow", dir)
	allowCmd.Dir = dir
	allowCmd.Env = append(os.Environ(), "PATH="+pathEnv)
	if out, err := allowCmd.CombinedOutput(); err != nil {
		t.Fatalf("direnv allow: %v\n%s", err, out)
	}

	outFile := filepath.Join(dir, "out.txt")
	execCmd2 := exec.Command(direnvBin, "exec", dir, "/bin/sh", "-c",
		fmt.Sprintf(`printf '%%s' "$DIRENV_TEST_KEY" > '%s'`, outFile))
	execCmd2.Dir = dir
	execCmd2.Env = append(os.Environ(), "PATH="+pathEnv)
	if out, err := execCmd2.CombinedOutput(); err != nil {
		t.Fatalf("direnv exec: %v\n%s", err, out)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(data) != "direnv_works" {
		t.Errorf("expected 'direnv_works', got %q", string(data))
	}
}

func TestDirenvIntegration_RealDirenv_ProfileSwitch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}

	direnvBin, err := exec.LookPath("direnv")
	if err != nil {
		t.Skip("skipping: direnv not installed")
	}

	bin := ensureTestBinary(t)
	dir := t.TempDir()

	cfgContent := "project: testproject\nactive_profile: staging\nprofiles:\n  staging:\n    env_file: .env.staging\n"
	writeFile(t, dir, config.FullFileName, cfgContent)
	writeFile(t, dir, ".env", "DB_HOST=prod-db\n")
	writeFile(t, dir, ".env.staging", "DB_HOST=staging-db\n")
	writeFile(t, dir, ".envrc", `eval "$(envref resolve --direnv 2>/dev/null)" || true`)

	binDir := filepath.Dir(bin)
	pathEnv := binDir + ":" + os.Getenv("PATH")

	allowCmd := exec.Command(direnvBin, "allow", dir)
	allowCmd.Dir = dir
	allowCmd.Env = append(os.Environ(), "PATH="+pathEnv)
	if out, err := allowCmd.CombinedOutput(); err != nil {
		t.Fatalf("direnv allow: %v\n%s", err, out)
	}

	outFile := filepath.Join(dir, "out.txt")
	execCmd2 := exec.Command(direnvBin, "exec", dir, "/bin/sh", "-c",
		fmt.Sprintf(`printf '%%s' "$DB_HOST" > '%s'`, outFile))
	execCmd2.Dir = dir
	execCmd2.Env = append(os.Environ(), "PATH="+pathEnv)
	if out, err := execCmd2.CombinedOutput(); err != nil {
		t.Fatalf("direnv exec: %v\n%s", err, out)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(data) != "staging-db" {
		t.Errorf("expected 'staging-db', got %q", string(data))
	}
}

// --- Edge Cases ---

func TestDirenvIntegration_EmptyEnv_NoExports(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := mkDirenvProject(t, "testproject", "# only comments\n\n", "")

	stdout, _, err := shellRun(t, dir, bin+" resolve --direnv", bin)
	if err != nil {
		t.Fatalf("resolve --direnv: %v", err)
	}

	if strings.Contains(stdout, "export") {
		t.Errorf("empty env should produce no exports, got:\n%s", stdout)
	}
}

func TestDirenvIntegration_ManyVars_AllInjected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)

	// Generate an env file with 50 variables.
	var envContent strings.Builder
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&envContent, "VAR_%03d=value_%03d\n", i, i)
	}

	dir := mkDirenvProject(t, "testproject", envContent.String(), "")

	// Count exports in output.
	stdout, _, err := shellRun(t, dir, bin+" resolve --direnv", bin)
	if err != nil {
		t.Fatalf("resolve --direnv: %v", err)
	}

	exportCount := strings.Count(stdout, "export ")
	if exportCount != 50 {
		t.Errorf("expected 50 exports, got %d", exportCount)
	}

	// Eval and verify a sample var.
	outFile := filepath.Join(dir, "out.txt")
	script := fmt.Sprintf(
		`eval "$(%s resolve --direnv)" && printf '%%s' "$VAR_025" > '%s'`, bin, outFile)
	_, _, err = shellRun(t, dir, script, bin)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(data) != "value_025" {
		t.Errorf("expected 'value_025', got %q", string(data))
	}
}

func TestDirenvIntegration_FormatShell_EquivalentToDirenv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}
	bin := ensureTestBinary(t)
	dir := mkDirenvProject(t, "testproject", "A=1\nB=hello world\n", "")

	// --direnv and --format shell should produce identical output.
	direnvOut, _, err := shellRun(t, dir, bin+" resolve --direnv", bin)
	if err != nil {
		t.Fatalf("--direnv: %v", err)
	}

	shellOut, _, err := shellRun(t, dir, bin+" resolve --format shell", bin)
	if err != nil {
		t.Fatalf("--format shell: %v", err)
	}

	if direnvOut != shellOut {
		t.Errorf("--direnv and --format shell should be identical:\n  --direnv: %q\n  --format shell: %q", direnvOut, shellOut)
	}
}
