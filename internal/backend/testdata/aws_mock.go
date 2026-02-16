// aws_mock is a test helper that mimics the AWS CLI for testing
// the AWSSSMBackend. It is built and used by awsssm_test.go.
//
// Usage: aws_mock ssm get-parameter|put-parameter|delete-parameter|describe-parameters [args...]
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
		fatal("usage: aws_mock ssm <subcommand> [args...]")
	}

	if args[0] != "ssm" {
		fatal("Unknown service: %s", args[0])
	}

	subcmd := args[1]
	rest := args[2:]

	store := loadStore()

	switch subcmd {
	case "get-parameter":
		handleGetParameter(store, rest)
	case "put-parameter":
		handlePutParameter(store, rest)
	case "delete-parameter":
		handleDeleteParameter(store, rest)
	case "describe-parameters":
		handleDescribeParameters(store, rest)
	default:
		fatal("Unknown operation: %s", subcmd)
	}
}

func handleGetParameter(store map[string]string, args []string) {
	name := flagValue(args, "--name", "")
	if name == "" {
		fatal("An error occurred (MissingParameterException) when calling the GetParameter operation: parameter name is required")
	}

	val, ok := store[name]
	if !ok {
		fatal("An error occurred (ParameterNotFound) when calling the GetParameter operation: parameter %q does not exist", name)
	}

	resp := map[string]interface{}{
		"Parameter": map[string]interface{}{
			"Name":  name,
			"Value": val,
			"Type":  "SecureString",
		},
	}
	writeJSON(resp)
}

func handlePutParameter(store map[string]string, args []string) {
	name := flagValue(args, "--name", "")
	value := flagValue(args, "--value", "")

	if name == "" {
		fatal("An error occurred (MissingParameterException) when calling the PutParameter operation: parameter name is required")
	}

	store[name] = value
	saveStore(store)

	resp := map[string]interface{}{
		"Version": 1,
		"Tier":    "Standard",
	}
	writeJSON(resp)
}

func handleDeleteParameter(store map[string]string, args []string) {
	name := flagValue(args, "--name", "")
	if name == "" {
		fatal("An error occurred (MissingParameterException) when calling the DeleteParameter operation: parameter name is required")
	}

	_, ok := store[name]
	if !ok {
		fatal("An error occurred (ParameterNotFound) when calling the DeleteParameter operation: parameter %q does not exist", name)
	}

	delete(store, name)
	saveStore(store)

	// AWS CLI returns empty response on successful delete.
	fmt.Print("{}")
}

func handleDescribeParameters(store map[string]string, args []string) {
	// Parse the parameter-filters to get the prefix.
	filter := flagValue(args, "--parameter-filters", "")

	// Extract prefix from filter string "Key=Name,Option=BeginsWith,Values=/prefix/"
	prefix := ""
	if filter != "" {
		for _, part := range strings.Split(filter, ",") {
			if strings.HasPrefix(part, "Values=") {
				prefix = strings.TrimPrefix(part, "Values=")
			}
		}
	}

	// Collect matching keys.
	var params []map[string]string
	var names []string
	for k := range store {
		names = append(names, k)
	}
	sort.Strings(names)

	for _, name := range names {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			params = append(params, map[string]string{
				"Name": name,
				"Type": "SecureString",
			})
		}
	}

	if params == nil {
		params = []map[string]string{}
	}

	resp := map[string]interface{}{
		"Parameters": params,
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
	return filepath.Join(filepath.Dir(exe), "aws_store.json")
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
