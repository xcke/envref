// op_mock is a test helper that mimics the 1Password CLI (op) for testing
// the OnePasswordBackend. It is built and used by onepassword_test.go.
//
// Usage: op_mock item get|create|edit|delete|list [args...]
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

type item struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Fields []field `json:"fields,omitempty"`
}

type field struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

type listItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func main() {
	args := os.Args[1:]
	if len(args) < 2 {
		fatal("usage: op_mock item <subcommand> [args...]")
	}

	if args[0] != "item" {
		fatal("unknown command: %s", args[0])
	}

	subcmd := args[1]
	rest := args[2:]

	store := loadStore()

	switch subcmd {
	case "get":
		handleGet(store, rest)
	case "create":
		handleCreate(store, rest)
	case "edit":
		handleEdit(store, rest)
	case "delete":
		handleDelete(store, rest)
	case "list":
		handleList(store, rest)
	default:
		fatal("unknown subcommand: %s", subcmd)
	}
}

func handleGet(store map[string]string, args []string) {
	if len(args) == 0 {
		fatal("[ERROR] 2024/01/01 00:00:00 no item specified")
	}
	title := args[0]

	val, ok := store[title]
	if !ok {
		fatal("[ERROR] 2024/01/01 00:00:00 \"%s\" isn't an item in the \"%s\" vault", title, flagValue(args, "--vault", "Personal"))
	}

	resp := item{
		ID:    "mock-id-" + title,
		Title: title,
		Fields: []field{
			{ID: "notesPlain", Label: "notesPlain", Value: val, Type: "STRING"},
		},
	}
	writeJSON(resp)
}

func handleCreate(store map[string]string, args []string) {
	title := flagValue(args, "--title", "")
	if title == "" {
		fatal("[ERROR] 2024/01/01 00:00:00 --title is required")
	}

	// Extract notesPlain= assignment from positional args.
	value := ""
	for _, a := range args {
		if strings.HasPrefix(a, "notesPlain=") {
			value = strings.TrimPrefix(a, "notesPlain=")
		}
	}

	store[title] = value
	saveStore(store)

	// op create outputs the created item.
	resp := item{
		ID:    "mock-id-" + title,
		Title: title,
		Fields: []field{
			{ID: "notesPlain", Label: "notesPlain", Value: value, Type: "STRING"},
		},
	}
	writeJSON(resp)
}

func handleEdit(store map[string]string, args []string) {
	if len(args) == 0 {
		fatal("[ERROR] 2024/01/01 00:00:00 no item specified")
	}
	title := args[0]

	_, ok := store[title]
	if !ok {
		fatal("[ERROR] 2024/01/01 00:00:00 \"%s\" isn't an item in the \"%s\" vault", title, flagValue(args, "--vault", "Personal"))
	}

	// Extract notesPlain= assignment from positional args.
	for _, a := range args {
		if strings.HasPrefix(a, "notesPlain=") {
			store[title] = strings.TrimPrefix(a, "notesPlain=")
		}
	}

	saveStore(store)
}

func handleDelete(store map[string]string, args []string) {
	if len(args) == 0 {
		fatal("[ERROR] 2024/01/01 00:00:00 no item specified")
	}
	title := args[0]

	_, ok := store[title]
	if !ok {
		fatal("[ERROR] 2024/01/01 00:00:00 \"%s\" isn't an item in the \"%s\" vault", title, flagValue(args, "--vault", "Personal"))
	}

	delete(store, title)
	saveStore(store)
}

func handleList(store map[string]string, args []string) {
	items := make([]listItem, 0, len(store))
	for k := range store {
		items = append(items, listItem{ID: "mock-id-" + k, Title: k})
	}
	// Sort for deterministic output.
	sort.Slice(items, func(i, j int) bool {
		return items[i].Title < items[j].Title
	})
	writeJSON(items)
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
	return filepath.Join(filepath.Dir(exe), "op_store.json")
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
	os.Exit(1)
}
