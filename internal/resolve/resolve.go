// Package resolve implements the reference resolution pipeline for envref.
//
// The pipeline takes merged and interpolated environment variables, identifies
// ref:// references, resolves them via the configured secret backends, and
// returns fully resolved key-value pairs ready for shell export.
package resolve

import (
	"errors"
	"fmt"
	"strings"

	"github.com/xcke/envref/internal/backend"
	"github.com/xcke/envref/internal/envfile"
	"github.com/xcke/envref/internal/ref"
)

// Result holds the output of a resolution pass.
type Result struct {
	// Entries contains all resolved key-value pairs in order.
	Entries []Entry
	// Errors contains per-key resolution failures.
	Errors []KeyErr
}

// Entry is a single resolved environment variable.
type Entry struct {
	// Key is the variable name.
	Key string
	// Value is the resolved value (secret value for refs, original for non-refs).
	Value string
	// WasRef indicates whether this entry was a ref:// reference that was resolved.
	WasRef bool
}

// KeyErr records a resolution failure for a specific key.
type KeyErr struct {
	// Key is the environment variable name that failed to resolve.
	Key string
	// Ref is the original ref:// URI.
	Ref string
	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e KeyErr) Error() string {
	return fmt.Sprintf("%s: failed to resolve %s: %v", e.Key, e.Ref, e.Err)
}

// Resolved returns true if the result has no resolution errors.
func (r *Result) Resolved() bool {
	return len(r.Errors) == 0
}

// Resolve takes a merged and interpolated Env and resolves all ref:// references
// using the provided registry. Each ref:// value is parsed to extract the backend
// name and key path; if the ref specifies a known backend name, that backend is
// queried directly; otherwise, the registry's ordered fallback is used.
//
// The project parameter is used to namespace secret lookups via NamespacedBackend.
//
// Non-ref entries are passed through unchanged. Resolution errors are collected
// per-key rather than failing the entire operation.
func Resolve(env *envfile.Env, registry *backend.Registry, project string) (*Result, error) {
	return ResolveWithProfile(env, registry, project, "")
}

// ResolveWithProfile works like Resolve but supports profile-scoped secrets.
// When profile is non-empty, each ref:// lookup first tries the profile-scoped
// namespace (<project>/<profile>/<key>), then falls back to the project-scoped
// namespace (<project>/<key>). This allows profile-specific secret overrides
// while maintaining project-wide defaults.
//
// When profile is empty, behavior is identical to Resolve.
func ResolveWithProfile(env *envfile.Env, registry *backend.Registry, project, profile string) (*Result, error) {
	if env == nil {
		return nil, fmt.Errorf("env must not be nil")
	}
	if registry == nil {
		return nil, fmt.Errorf("registry must not be nil")
	}
	if project == "" {
		return nil, fmt.Errorf("project name must not be empty")
	}

	// Build project-scoped namespaced wrappers for each backend.
	nsBackends := make(map[string]*backend.NamespacedBackend)
	for _, b := range registry.Backends() {
		ns, err := backend.NewNamespacedBackend(b, project)
		if err != nil {
			return nil, fmt.Errorf("wrapping backend %q: %w", b.Name(), err)
		}
		nsBackends[b.Name()] = ns
	}

	// Build a project-scoped namespaced registry for fallback resolution.
	nsRegistry := backend.NewRegistry()
	for _, name := range registry.Names() {
		if err := nsRegistry.Register(nsBackends[name]); err != nil {
			return nil, fmt.Errorf("registering namespaced backend %q: %w", name, err)
		}
	}

	// Build profile-scoped namespaced wrappers if a profile is active.
	var profileBackends map[string]*backend.NamespacedBackend
	var profileRegistry *backend.Registry
	if profile != "" {
		profileBackends = make(map[string]*backend.NamespacedBackend)
		for _, b := range registry.Backends() {
			ns, err := backend.NewProfileNamespacedBackend(b, project, profile)
			if err != nil {
				return nil, fmt.Errorf("wrapping backend %q for profile %q: %w", b.Name(), profile, err)
			}
			profileBackends[b.Name()] = ns
		}

		profileRegistry = backend.NewRegistry()
		for _, name := range registry.Names() {
			if err := profileRegistry.Register(profileBackends[name]); err != nil {
				return nil, fmt.Errorf("registering profile backend %q: %w", name, err)
			}
		}
	}

	// Cache resolved values to avoid duplicate backend hits when multiple
	// env vars reference the same secret (keyed by raw ref:// URI).
	type cachedResult struct {
		value string
		err   error
	}
	cache := make(map[string]cachedResult)

	result := &Result{}
	for _, envEntry := range env.All() {
		if !envEntry.IsRef {
			result.Entries = append(result.Entries, Entry{
				Key:    envEntry.Key,
				Value:  envEntry.Value,
				WasRef: false,
			})
			continue
		}

		// Parse the ref:// URI.
		parsed, err := ref.Parse(envEntry.Value)
		if err != nil {
			result.Errors = append(result.Errors, KeyErr{
				Key: envEntry.Key,
				Ref: envEntry.Value,
				Err: fmt.Errorf("invalid ref:// URI: %w", err),
			})
			result.Entries = append(result.Entries, Entry{
				Key:    envEntry.Key,
				Value:  envEntry.Value,
				WasRef: true,
			})
			continue
		}

		// Check the cache before hitting backends.
		cached, ok := cache[envEntry.Value]
		if !ok {
			var value string
			var resolveErr error

			// If a profile is active, try profile-scoped first.
			if profileBackends != nil {
				value, resolveErr = resolveRef(parsed, profileBackends, profileRegistry)
			}

			// Fall back to project-scoped if no profile or profile lookup failed with not-found.
			if profileBackends == nil || isNotFoundError(resolveErr) {
				value, resolveErr = resolveRef(parsed, nsBackends, nsRegistry)
			}

			cached = cachedResult{value: value, err: resolveErr}
			cache[envEntry.Value] = cached
		}

		if cached.err != nil {
			result.Errors = append(result.Errors, KeyErr{
				Key: envEntry.Key,
				Ref: envEntry.Value,
				Err: cached.err,
			})
			// Keep the original ref:// value for unresolved entries.
			result.Entries = append(result.Entries, Entry{
				Key:    envEntry.Key,
				Value:  envEntry.Value,
				WasRef: true,
			})
			continue
		}

		result.Entries = append(result.Entries, Entry{
			Key:    envEntry.Key,
			Value:  cached.value,
			WasRef: true,
		})
	}

	return result, nil
}

// isNotFoundError returns true if the error indicates a secret was not found.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, backend.ErrNotFound) ||
		strings.Contains(err.Error(), "not found")
}

// resolveRef looks up a parsed reference in the backends. If the ref specifies
// a backend name that matches a registered backend, it queries that backend
// directly. Otherwise, it uses the registry's fallback chain with the ref path
// as the key.
func resolveRef(parsed ref.Reference, nsBackends map[string]*backend.NamespacedBackend, nsRegistry *backend.Registry) (string, error) {
	// If the ref backend name matches a registered backend, query it directly.
	if ns, ok := nsBackends[parsed.Backend]; ok {
		value, err := ns.Get(parsed.Path)
		if err != nil {
			if errors.Is(err, backend.ErrNotFound) {
				return "", fmt.Errorf("secret %q not found in backend %q", parsed.Path, parsed.Backend)
			}
			return "", fmt.Errorf("backend %q: %w", parsed.Backend, err)
		}
		return value, nil
	}

	// For generic backend names (like "secrets"), try the fallback chain.
	value, err := nsRegistry.Get(parsed.Path)
	if err != nil {
		if errors.Is(err, backend.ErrNotFound) {
			return "", fmt.Errorf("secret %q not found in any backend", parsed.Path)
		}
		return "", err
	}
	return value, nil
}
