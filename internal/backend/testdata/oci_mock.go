// oci_mock is a test helper that mimics the OCI CLI for testing
// the OCIVaultBackend. It is built and used by ocivault_test.go.
//
// Usage: oci_mock vault secret create-base64|update-base64|list|schedule-secret-deletion [args...]
//        oci_mock secrets secret-bundle get [args...]
//
// State is persisted in a JSON file in the executable's directory so that
// multiple invocations maintain consistent state within a single test.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type secretEntry struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"` // base64-encoded
	State   string `json:"state"`
}

func main() {
	args := os.Args[1:]
	if len(args) < 2 {
		fatal("ServiceError: usage: oci_mock <service> <subcommand> [args...]")
	}

	store := loadStore()

	service := args[0]
	switch service {
	case "vault":
		handleVault(store, args[1:])
	case "secrets":
		handleSecrets(store, args[1:])
	default:
		fatal("ServiceError: unknown service: %s", service)
	}
}

func handleVault(store map[string]*secretEntry, args []string) {
	if len(args) < 2 {
		fatal("ServiceError: usage: oci vault secret <subcommand> [args...]")
	}
	if args[0] != "secret" {
		fatal("ServiceError: unknown vault subcommand: %s", args[0])
	}

	subcmd := args[1]
	rest := args[2:]

	switch subcmd {
	case "create-base64":
		handleCreate(store, rest)
	case "update-base64":
		handleUpdate(store, rest)
	case "list":
		handleList(store, rest)
	case "schedule-secret-deletion":
		handleDelete(store, rest)
	default:
		fatal("ServiceError: unknown secret subcommand: %s", subcmd)
	}
}

func handleSecrets(store map[string]*secretEntry, args []string) {
	if len(args) < 2 {
		fatal("ServiceError: usage: oci secrets secret-bundle get [args...]")
	}
	if args[0] != "secret-bundle" || args[1] != "get" {
		fatal("ServiceError: unknown secrets subcommand: %s %s", args[0], args[1])
	}
	handleGetBundle(store, args[2:])
}

func handleCreate(store map[string]*secretEntry, args []string) {
	name := flagValue(args, "--secret-name", "")
	content := flagValue(args, "--secret-content-content", "")

	if name == "" {
		fatal("ServiceError: --secret-name is required")
	}

	id := "ocid1.secret.mock." + name
	store[name] = &secretEntry{
		ID:      id,
		Name:    name,
		Content: content,
		State:   "ACTIVE",
	}
	saveStore(store)

	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"id":          id,
			"secret-name": name,
		},
	}
	writeJSON(resp)
}

func handleUpdate(store map[string]*secretEntry, args []string) {
	secretID := flagValue(args, "--secret-id", "")
	content := flagValue(args, "--secret-content-content", "")

	if secretID == "" {
		fatal("ServiceError: --secret-id is required")
	}

	// Find by ID.
	var found *secretEntry
	for _, e := range store {
		if e.ID == secretID {
			found = e
			break
		}
	}
	if found == nil {
		fatal("ServiceError: 404 NotFound: secret with id %q not found", secretID)
	}

	found.Content = content
	saveStore(store)

	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"id":          found.ID,
			"secret-name": found.Name,
		},
	}
	writeJSON(resp)
}

func handleList(store map[string]*secretEntry, args []string) {
	nameFilter := flagValue(args, "--name", "")
	stateFilter := flagValue(args, "--lifecycle-state", "")

	var items []map[string]interface{}
	var names []string
	for k := range store {
		names = append(names, k)
	}
	sort.Strings(names)

	for _, k := range names {
		e := store[k]
		if stateFilter != "" && e.State != stateFilter {
			continue
		}
		if nameFilter != "" && e.Name != nameFilter {
			continue
		}
		items = append(items, map[string]interface{}{
			"id":              e.ID,
			"secret-name":     e.Name,
			"lifecycle-state": e.State,
		})
	}

	if items == nil {
		items = []map[string]interface{}{}
	}

	resp := map[string]interface{}{
		"data": items,
	}
	writeJSON(resp)
}

func handleDelete(store map[string]*secretEntry, args []string) {
	secretID := flagValue(args, "--secret-id", "")

	if secretID == "" {
		fatal("ServiceError: --secret-id is required")
	}

	// Find by ID and mark as PENDING_DELETION.
	var found *secretEntry
	for _, e := range store {
		if e.ID == secretID {
			found = e
			break
		}
	}
	if found == nil {
		fatal("ServiceError: 404 NotFound: secret with id %q not found", secretID)
	}

	found.State = "PENDING_DELETION"
	saveStore(store)

	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"id":    found.ID,
			"state": "PENDING_DELETION",
		},
	}
	writeJSON(resp)
}

func handleGetBundle(store map[string]*secretEntry, args []string) {
	secretID := flagValue(args, "--secret-id", "")

	if secretID == "" {
		fatal("ServiceError: --secret-id is required")
	}

	// Find by ID.
	var found *secretEntry
	for _, e := range store {
		if e.ID == secretID {
			found = e
			break
		}
	}
	if found == nil {
		fatal("ServiceError: 404 NotFound: secret bundle for id %q not found", secretID)
	}

	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"secret-bundle-content": map[string]interface{}{
				"content":      found.Content,
				"content-type": "BASE64",
			},
		},
	}
	writeJSON(resp)
}

// flagValue extracts the value of a --flag from args. Returns def if not found.
func flagValue(args []string, flag, def string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return def
}

func storePath() string {
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "oci_store.json")
}

func loadStore() map[string]*secretEntry {
	data, err := os.ReadFile(storePath())
	if err != nil {
		return make(map[string]*secretEntry)
	}
	var store map[string]*secretEntry
	if err := json.Unmarshal(data, &store); err != nil {
		return make(map[string]*secretEntry)
	}
	return store
}

func saveStore(store map[string]*secretEntry) {
	data, _ := json.Marshal(store)
	_ = os.WriteFile(storePath(), data, 0o644)
}

func writeJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(v)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
