// vault_mock is a test helper that mimics the HashiCorp Vault CLI for testing
// the HashiVaultBackend. It is built and used by hashivault_test.go.
//
// Usage: vault_mock kv get|put|list|metadata [args...]
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
	"strings"
)

func main() {
	args := os.Args[1:]
	if len(args) < 2 {
		fatal("usage: vault_mock kv <subcommand> [args...]")
	}

	if args[0] != "kv" {
		fatal("Error: unknown command %q", args[0])
	}

	subcmd := args[1]
	rest := args[2:]

	store := loadStore()

	switch subcmd {
	case "get":
		handleKVGet(store, rest)
	case "put":
		handleKVPut(store, rest)
	case "list":
		handleKVList(store, rest)
	case "metadata":
		if len(rest) > 0 && rest[0] == "delete" {
			handleKVMetadataDelete(store, rest[1:])
		} else {
			fatal("Error: unknown metadata subcommand")
		}
	default:
		fatal("Error: unknown kv subcommand %q", subcmd)
	}
}

func handleKVGet(store map[string]string, args []string) {
	mount, rest := extractFlag(args, "-mount")
	_, rest = extractFlag(rest, "-format")
	// Remaining args: global flags and the path.
	_, rest = extractFlag(rest, "-address")
	_, rest = extractFlag(rest, "-namespace")

	if len(rest) == 0 {
		fatal("Error: not enough arguments")
	}
	path := rest[len(rest)-1]

	// Build full key from mount + path.
	fullKey := mount + "/" + path

	val, ok := store[fullKey]
	if !ok {
		fatal("No value found at %s", mount+"/data/"+path)
	}

	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"data": map[string]interface{}{
				"value": val,
			},
			"metadata": map[string]interface{}{
				"version": 1,
			},
		},
	}
	writeJSON(resp)
}

func handleKVPut(store map[string]string, args []string) {
	mount, rest := extractFlag(args, "-mount")
	_, rest = extractFlag(rest, "-format")
	_, rest = extractFlag(rest, "-address")
	_, rest = extractFlag(rest, "-namespace")

	if len(rest) < 2 {
		fatal("Error: not enough arguments (need path and key=value)")
	}

	path := rest[0]
	// Parse key=value pairs from remaining args.
	var value string
	for _, arg := range rest[1:] {
		if strings.HasPrefix(arg, "value=") {
			value = strings.TrimPrefix(arg, "value=")
		}
	}

	fullKey := mount + "/" + path
	store[fullKey] = value
	saveStore(store)

	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"created_time": "2024-01-01T00:00:00Z",
			"version":      1,
		},
	}
	writeJSON(resp)
}

func handleKVList(store map[string]string, args []string) {
	mount, rest := extractFlag(args, "-mount")
	_, rest = extractFlag(rest, "-format")
	_, rest = extractFlag(rest, "-address")
	_, rest = extractFlag(rest, "-namespace")

	if len(rest) == 0 {
		fatal("Error: not enough arguments")
	}
	prefix := rest[len(rest)-1]

	// Normalize: ensure prefix ends with / for matching.
	fullPrefix := mount + "/" + prefix
	if !strings.HasSuffix(fullPrefix, "/") {
		fullPrefix += "/"
	}

	var keys []string
	seen := make(map[string]bool)
	var allStoreKeys []string
	for k := range store {
		allStoreKeys = append(allStoreKeys, k)
	}
	sort.Strings(allStoreKeys)

	for _, k := range allStoreKeys {
		if strings.HasPrefix(k, fullPrefix) {
			// Strip the prefix to get the relative key name.
			relKey := strings.TrimPrefix(k, fullPrefix)
			if relKey != "" && !seen[relKey] {
				keys = append(keys, relKey)
				seen[relKey] = true
			}
		}
	}

	if len(keys) == 0 {
		fatal("No value found at %s/metadata/%s", mount, prefix)
	}

	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"keys": keys,
		},
	}
	writeJSON(resp)
}

func handleKVMetadataDelete(store map[string]string, args []string) {
	mount, rest := extractFlag(args, "-mount")
	_, rest = extractFlag(rest, "-address")
	_, rest = extractFlag(rest, "-namespace")

	if len(rest) == 0 {
		fatal("Error: not enough arguments")
	}
	path := rest[len(rest)-1]

	fullKey := mount + "/" + path
	_, ok := store[fullKey]
	if !ok {
		fatal("No value found at %s/metadata/%s", mount, path)
	}

	delete(store, fullKey)
	saveStore(store)

	// Vault CLI returns "Success! Data deleted (if it existed) at: ..." on stderr.
	// stdout is empty on success.
	fmt.Print("{}")
}

// extractFlag extracts a flag like "-mount=value" or "-mount value" from args.
// Returns the value and the remaining args with the flag removed.
func extractFlag(args []string, flag string) (string, []string) {
	var remaining []string
	var value string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		// Handle -flag=value format.
		if strings.HasPrefix(arg, flag+"=") {
			value = strings.TrimPrefix(arg, flag+"=")
			continue
		}
		// Handle -flag value format.
		if arg == flag && i+1 < len(args) {
			value = args[i+1]
			i++ // skip next arg
			continue
		}
		remaining = append(remaining, arg)
	}

	return value, remaining
}

func storePath() string {
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "vault_store.json")
}

func loadStore() map[string]string {
	data, err := os.ReadFile(storePath())
	if err != nil {
		return make(map[string]string)
	}
	var store map[string]string
	if err := json.Unmarshal(data, &store); err != nil {
		return make(map[string]string)
	}
	return store
}

func saveStore(store map[string]string) {
	data, _ := json.Marshal(store)
	_ = os.WriteFile(storePath(), data, 0o644)
}

func writeJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(v)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
