// plugin_helper is a test plugin executable that implements the envref
// backend plugin protocol with a file-backed JSON store. It is built and
// used by plugin_test.go.
//
// Usage: plugin_helper serve
//
// Reads a JSON request from stdin, processes it, and writes a JSON response
// to stdout. State is persisted in a temporary JSON file so that multiple
// invocations (set, get, list, delete) maintain consistent state within a
// single test.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type request struct {
	Operation string `json:"operation"`
	Key       string `json:"key,omitempty"`
	Value     string `json:"value,omitempty"`
}

type response struct {
	Value string   `json:"value,omitempty"`
	Keys  []string `json:"keys,omitempty"`
	Error string   `json:"error,omitempty"`
}

func main() {
	if len(os.Args) < 2 || os.Args[1] != "serve" {
		fmt.Fprintf(os.Stderr, "usage: %s serve\n", os.Args[0])
		os.Exit(1)
	}

	var req request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		writeResponse(response{Error: fmt.Sprintf("invalid request: %v", err)})
		return
	}

	store := loadStore()
	var resp response

	switch req.Operation {
	case "get":
		val, ok := store[req.Key]
		if !ok {
			resp.Error = "not found"
		} else {
			resp.Value = val
		}
	case "set":
		store[req.Key] = req.Value
		saveStore(store)
	case "delete":
		if _, ok := store[req.Key]; !ok {
			resp.Error = "not found"
		} else {
			delete(store, req.Key)
			saveStore(store)
		}
	case "list":
		keys := make([]string, 0, len(store))
		for k := range store {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		resp.Keys = keys
	default:
		resp.Error = fmt.Sprintf("unknown operation: %s", req.Operation)
	}

	writeResponse(resp)
}

func storePath() string {
	// Use the executable's directory for state, so each test gets
	// its own store (since we build to a unique temp dir).
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "store.json")
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

func writeResponse(resp response) {
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}
