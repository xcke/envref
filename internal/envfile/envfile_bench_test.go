package envfile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xcke/envref/internal/envfile"
)

// writeEnvFile creates a temporary .env file with n key-value entries.
func writeEnvFile(b *testing.B, dir string, name string, n int) string {
	b.Helper()
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString("KEY_")
		sb.WriteString(string(rune('A' + i%26)))
		sb.WriteString(string(rune('0' + (i/26)%10)))
		sb.WriteString("=value_")
		sb.WriteString(string(rune('0' + i%10)))
		sb.WriteByte('\n')
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		b.Fatal(err)
	}
	return path
}

func BenchmarkLoad50(b *testing.B) {
	dir := b.TempDir()
	path := writeEnvFile(b, dir, ".env", 50)
	b.ResetTimer()
	for b.Loop() {
		_, _, _ = envfile.Load(path)
	}
}

func BenchmarkLoad100(b *testing.B) {
	dir := b.TempDir()
	path := writeEnvFile(b, dir, ".env", 100)
	b.ResetTimer()
	for b.Loop() {
		_, _, _ = envfile.Load(path)
	}
}

func BenchmarkMerge(b *testing.B) {
	dir := b.TempDir()
	basePath := writeEnvFile(b, dir, ".env", 50)
	localPath := writeEnvFile(b, dir, ".env.local", 10)
	base, _, _ := envfile.Load(basePath)
	local, _, _ := envfile.Load(localPath)
	b.ResetTimer()
	for b.Loop() {
		_ = envfile.Merge(base, local)
	}
}

func BenchmarkInterpolate(b *testing.B) {
	dir := b.TempDir()
	// Create a file with interpolation-heavy entries.
	var sb strings.Builder
	sb.WriteString("HOST=localhost\n")
	sb.WriteString("PORT=5432\n")
	sb.WriteString("DB_NAME=myapp\n")
	for i := 0; i < 47; i++ {
		sb.WriteString("URL_")
		sb.WriteString(string(rune('A' + i%26)))
		sb.WriteString(string(rune('0' + (i/26)%10)))
		sb.WriteString("=postgres://${HOST}:${PORT}/${DB_NAME}\n")
	}
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		b.Fatal(err)
	}
	env, _, _ := envfile.Load(path)
	b.ResetTimer()
	for b.Loop() {
		// We need a fresh copy since Interpolate modifies in place.
		clone := envfile.Merge(env)
		envfile.Interpolate(clone)
	}
}

func BenchmarkLoadMergeInterpolate(b *testing.B) {
	dir := b.TempDir()
	basePath := writeEnvFile(b, dir, ".env", 80)
	localPath := writeEnvFile(b, dir, ".env.local", 20)
	b.ResetTimer()
	for b.Loop() {
		base, _, _ := envfile.Load(basePath)
		local, _, _ := envfile.Load(localPath)
		merged := envfile.Merge(base, local)
		envfile.Interpolate(merged)
	}
}
