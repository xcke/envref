// Package schema provides JSON-based schema validation for .env files.
//
// A schema file (.env.schema.json) defines validation rules for each
// environment variable key, including type constraints, required status,
// default values, and allowed values (enums).
//
// Example .env.schema.json:
//
//	{
//	  "keys": {
//	    "DB_HOST": { "type": "string", "required": true, "description": "Database hostname" },
//	    "DB_PORT": { "type": "port", "required": true, "default": "5432" },
//	    "DEBUG":   { "type": "boolean" },
//	    "LOG_LEVEL": { "type": "enum", "values": ["debug", "info", "warn", "error"] },
//	    "API_URL": { "type": "url", "required": true }
//	  }
//	}
package schema

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Schema represents a parsed .env.schema.json file.
type Schema struct {
	// Keys maps environment variable names to their validation rules.
	Keys map[string]Rule `json:"keys"`
}

// Rule defines validation constraints for a single environment variable.
type Rule struct {
	// Type is the expected value type: string, number, boolean, url, enum, email, port.
	// Defaults to "string" if empty.
	Type string `json:"type,omitempty"`
	// Required indicates whether the key must be present.
	Required bool `json:"required,omitempty"`
	// Default is the default value (informational; not applied during validation).
	Default string `json:"default,omitempty"`
	// Values lists allowed values when Type is "enum".
	Values []string `json:"values,omitempty"`
	// Pattern is an optional regex pattern the value must match.
	Pattern string `json:"pattern,omitempty"`
	// Description documents the purpose of this variable.
	Description string `json:"description,omitempty"`
}

// ValidationError represents a single validation failure for a key.
type ValidationError struct {
	Key     string
	Message string
}

func (e ValidationError) String() string {
	return fmt.Sprintf("%s: %s", e.Key, e.Message)
}

// Result holds the outcome of validating an environment against a schema.
type Result struct {
	Errors []ValidationError
}

// OK returns true if no validation errors were found.
func (r *Result) OK() bool {
	return len(r.Errors) == 0
}

// Load reads and parses a .env.schema.json file from disk.
func Load(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading schema file: %w", err)
	}
	return Parse(data)
}

// Parse decodes JSON data into a Schema.
func Parse(data []byte) (*Schema, error) {
	var s Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing schema JSON: %w", err)
	}
	if s.Keys == nil {
		s.Keys = make(map[string]Rule)
	}
	// Validate the schema itself.
	if err := s.validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

// validate checks that the schema definition is well-formed.
func (s *Schema) validate() error {
	validTypes := map[string]bool{
		"":        true, // defaults to string
		"string":  true,
		"number":  true,
		"boolean": true,
		"url":     true,
		"enum":    true,
		"email":   true,
		"port":    true,
	}

	for key, rule := range s.Keys {
		if !validTypes[rule.Type] {
			return fmt.Errorf("schema error: key %q has unknown type %q", key, rule.Type)
		}
		if rule.Type == "enum" && len(rule.Values) == 0 {
			return fmt.Errorf("schema error: key %q has type \"enum\" but no values defined", key)
		}
		if rule.Pattern != "" {
			if _, err := regexp.Compile(rule.Pattern); err != nil {
				return fmt.Errorf("schema error: key %q has invalid pattern %q: %w", key, rule.Pattern, err)
			}
		}
	}
	return nil
}

// Validate checks a set of key-value pairs against the schema rules.
// The entries parameter maps environment variable names to their resolved values.
// Returns a Result containing any validation errors found.
func (s *Schema) Validate(entries map[string]string) *Result {
	var errs []ValidationError

	// Check each key defined in the schema.
	keys := sortedKeys(s.Keys)
	for _, key := range keys {
		rule := s.Keys[key]
		value, exists := entries[key]

		// Check required.
		if rule.Required && !exists {
			errs = append(errs, ValidationError{
				Key:     key,
				Message: "required key is missing",
			})
			continue
		}

		if !exists {
			continue
		}

		// Empty values skip type checking (they may be intentionally blank).
		if value == "" {
			if rule.Required {
				errs = append(errs, ValidationError{
					Key:     key,
					Message: "required key has empty value",
				})
			}
			continue
		}

		// Type validation.
		if err := validateType(rule, value); err != nil {
			errs = append(errs, ValidationError{Key: key, Message: err.Error()})
		}

		// Pattern validation.
		if rule.Pattern != "" {
			matched, _ := regexp.MatchString(rule.Pattern, value)
			if !matched {
				errs = append(errs, ValidationError{
					Key:     key,
					Message: fmt.Sprintf("value %q does not match pattern %q", value, rule.Pattern),
				})
			}
		}
	}

	return &Result{Errors: errs}
}

// validateType checks that a value conforms to the expected type.
func validateType(rule Rule, value string) error {
	typ := rule.Type
	if typ == "" {
		typ = "string"
	}

	switch typ {
	case "string":
		// Any value is a valid string.
		return nil

	case "number":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("expected a number, got %q", value)
		}
		return nil

	case "boolean":
		lower := strings.ToLower(value)
		validBools := map[string]bool{
			"true": true, "false": true,
			"1": true, "0": true,
			"yes": true, "no": true,
			"on": true, "off": true,
		}
		if !validBools[lower] {
			return fmt.Errorf("expected a boolean (true/false/1/0/yes/no/on/off), got %q", value)
		}
		return nil

	case "url":
		u, err := url.Parse(value)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("expected a valid URL with scheme and host, got %q", value)
		}
		return nil

	case "enum":
		for _, allowed := range rule.Values {
			if value == allowed {
				return nil
			}
		}
		return fmt.Errorf("expected one of [%s], got %q", strings.Join(rule.Values, ", "), value)

	case "email":
		// Simple email validation: must contain exactly one @ with text on both sides.
		atIdx := strings.LastIndex(value, "@")
		if atIdx < 1 || atIdx >= len(value)-1 {
			return fmt.Errorf("expected a valid email address, got %q", value)
		}
		domain := value[atIdx+1:]
		if !strings.Contains(domain, ".") {
			return fmt.Errorf("expected a valid email address, got %q", value)
		}
		return nil

	case "port":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("expected a port number (1-65535), got %q", value)
		}
		return nil

	default:
		return fmt.Errorf("unknown type %q", typ)
	}
}

// sortedKeys returns the keys of the map in sorted order.
func sortedKeys(m map[string]Rule) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
