package resolve_test

import (
	"testing"

	"github.com/xcke/envref/internal/backend"
	"github.com/xcke/envref/internal/envfile"
	"github.com/xcke/envref/internal/parser"
	"github.com/xcke/envref/internal/resolve"
)

func buildBenchEnv(total, refCount int, project string) (*envfile.Env, *backend.Registry) {
	env := envfile.NewEnv()
	secrets := make(map[string]string)

	for i := 0; i < total; i++ {
		key := "KEY_" + string(rune('A'+i%26)) + string(rune('0'+(i/26)%10))
		if i < refCount {
			secretKey := "secret_" + string(rune('a'+i%26))
			env.Set(parser.Entry{
				Key:   key,
				Value: "ref://keychain/" + secretKey,
				IsRef: true,
			})
			// Pre-populate the mock backend with namespaced keys.
			secrets[project+"/"+secretKey] = "resolved_value_" + string(rune('0'+i%10))
		} else {
			env.Set(parser.Entry{
				Key:   key,
				Value: "plain_value_" + string(rune('0'+i%10)),
				IsRef: false,
			})
		}
	}

	registry := backend.NewRegistry()
	_ = registry.Register(newMockBackend("keychain", secrets))
	return env, registry
}

func BenchmarkResolve_NoRefs50(b *testing.B) {
	env, registry := buildBenchEnv(50, 0, "bench-project")
	b.ResetTimer()
	for b.Loop() {
		_, _ = resolve.Resolve(env, registry, "bench-project")
	}
}

func BenchmarkResolve_10Refs50(b *testing.B) {
	env, registry := buildBenchEnv(50, 10, "bench-project")
	b.ResetTimer()
	for b.Loop() {
		_, _ = resolve.Resolve(env, registry, "bench-project")
	}
}

func BenchmarkResolve_50Refs100(b *testing.B) {
	env, registry := buildBenchEnv(100, 50, "bench-project")
	b.ResetTimer()
	for b.Loop() {
		_, _ = resolve.Resolve(env, registry, "bench-project")
	}
}

func BenchmarkResolve_100Refs100(b *testing.B) {
	env, registry := buildBenchEnv(100, 100, "bench-project")
	b.ResetTimer()
	for b.Loop() {
		_, _ = resolve.Resolve(env, registry, "bench-project")
	}
}

func BenchmarkResolveWithProfile_10Refs50(b *testing.B) {
	env := envfile.NewEnv()
	secrets := make(map[string]string)

	project := "bench-project"
	profile := "staging"

	for i := 0; i < 50; i++ {
		key := "KEY_" + string(rune('A'+i%26)) + string(rune('0'+(i/26)%10))
		if i < 10 {
			secretKey := "secret_" + string(rune('a'+i%26))
			env.Set(parser.Entry{
				Key:   key,
				Value: "ref://keychain/" + secretKey,
				IsRef: true,
			})
			// Profile-scoped key.
			secrets[project+"/"+profile+"/"+secretKey] = "profile_value_" + string(rune('0'+i%10))
			// Project-scoped fallback.
			secrets[project+"/"+secretKey] = "project_value_" + string(rune('0'+i%10))
		} else {
			env.Set(parser.Entry{
				Key:   key,
				Value: "plain_value_" + string(rune('0'+i%10)),
				IsRef: false,
			})
		}
	}

	registry := backend.NewRegistry()
	_ = registry.Register(newMockBackend("keychain", secrets))
	b.ResetTimer()
	for b.Loop() {
		_, _ = resolve.ResolveWithProfile(env, registry, project, profile)
	}
}
