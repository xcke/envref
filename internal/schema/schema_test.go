package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_ValidSchema(t *testing.T) {
	data := []byte(`{
		"keys": {
			"DB_HOST": {"type": "string", "required": true, "description": "Database host"},
			"DB_PORT": {"type": "port", "required": true, "default": "5432"},
			"DEBUG": {"type": "boolean"},
			"LOG_LEVEL": {"type": "enum", "values": ["debug", "info", "warn", "error"]},
			"API_URL": {"type": "url", "required": true},
			"ADMIN_EMAIL": {"type": "email"},
			"MAX_RETRIES": {"type": "number"}
		}
	}`)

	s, err := Parse(data)
	require.NoError(t, err)
	assert.Len(t, s.Keys, 7)
	assert.Equal(t, "string", s.Keys["DB_HOST"].Type)
	assert.True(t, s.Keys["DB_HOST"].Required)
	assert.Equal(t, "5432", s.Keys["DB_PORT"].Default)
}

func TestParse_EmptyKeys(t *testing.T) {
	data := []byte(`{"keys": {}}`)
	s, err := Parse(data)
	require.NoError(t, err)
	assert.Empty(t, s.Keys)
}

func TestParse_NoKeysField(t *testing.T) {
	data := []byte(`{}`)
	s, err := Parse(data)
	require.NoError(t, err)
	assert.NotNil(t, s.Keys)
	assert.Empty(t, s.Keys)
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`{invalid`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing schema JSON")
}

func TestParse_UnknownType(t *testing.T) {
	data := []byte(`{"keys": {"FOO": {"type": "badtype"}}}`)
	_, err := Parse(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown type")
}

func TestParse_EnumWithoutValues(t *testing.T) {
	data := []byte(`{"keys": {"FOO": {"type": "enum"}}}`)
	_, err := Parse(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no values defined")
}

func TestParse_InvalidPattern(t *testing.T) {
	data := []byte(`{"keys": {"FOO": {"type": "string", "pattern": "[invalid"}}}`)
	_, err := Parse(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pattern")
}

func TestParse_DefaultTypeIsString(t *testing.T) {
	data := []byte(`{"keys": {"FOO": {}}}`)
	s, err := Parse(data)
	require.NoError(t, err)
	// Empty type defaults to string; should validate any value.
	result := s.Validate(map[string]string{"FOO": "anything"})
	assert.True(t, result.OK())
}

func TestValidate_RequiredKeyMissing(t *testing.T) {
	s := &Schema{Keys: map[string]Rule{
		"DB_HOST": {Type: "string", Required: true},
	}}

	result := s.Validate(map[string]string{})
	assert.False(t, result.OK())
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "DB_HOST", result.Errors[0].Key)
	assert.Contains(t, result.Errors[0].Message, "required key is missing")
}

func TestValidate_RequiredKeyEmpty(t *testing.T) {
	s := &Schema{Keys: map[string]Rule{
		"DB_HOST": {Type: "string", Required: true},
	}}

	result := s.Validate(map[string]string{"DB_HOST": ""})
	assert.False(t, result.OK())
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "required key has empty value")
}

func TestValidate_OptionalKeyMissing(t *testing.T) {
	s := &Schema{Keys: map[string]Rule{
		"OPTIONAL": {Type: "string"},
	}}

	result := s.Validate(map[string]string{})
	assert.True(t, result.OK())
}

func TestValidate_OptionalKeyEmpty(t *testing.T) {
	s := &Schema{Keys: map[string]Rule{
		"OPTIONAL": {Type: "number"},
	}}

	// Empty values skip type checking.
	result := s.Validate(map[string]string{"OPTIONAL": ""})
	assert.True(t, result.OK())
}

func TestValidate_StringType(t *testing.T) {
	s := &Schema{Keys: map[string]Rule{
		"NAME": {Type: "string"},
	}}

	result := s.Validate(map[string]string{"NAME": "anything goes"})
	assert.True(t, result.OK())
}

func TestValidate_NumberType(t *testing.T) {
	tests := []struct {
		name  string
		value string
		ok    bool
	}{
		{"integer", "42", true},
		{"negative", "-7", true},
		{"float", "3.14", true},
		{"scientific", "1.5e10", true},
		{"zero", "0", true},
		{"not a number", "abc", false},
		{"mixed", "42abc", false},
	}

	s := &Schema{Keys: map[string]Rule{
		"NUM": {Type: "number"},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Validate(map[string]string{"NUM": tt.value})
			if tt.ok {
				assert.True(t, result.OK(), "expected %q to be a valid number", tt.value)
			} else {
				assert.False(t, result.OK(), "expected %q to be invalid", tt.value)
				assert.Contains(t, result.Errors[0].Message, "expected a number")
			}
		})
	}
}

func TestValidate_BooleanType(t *testing.T) {
	validBools := []string{"true", "false", "True", "False", "TRUE", "FALSE", "1", "0", "yes", "no", "on", "off", "YES", "NO", "ON", "OFF"}
	invalidBools := []string{"maybe", "2", "truthy", "nope"}

	s := &Schema{Keys: map[string]Rule{
		"FLAG": {Type: "boolean"},
	}}

	for _, v := range validBools {
		t.Run("valid_"+v, func(t *testing.T) {
			result := s.Validate(map[string]string{"FLAG": v})
			assert.True(t, result.OK(), "expected %q to be a valid boolean", v)
		})
	}

	for _, v := range invalidBools {
		t.Run("invalid_"+v, func(t *testing.T) {
			result := s.Validate(map[string]string{"FLAG": v})
			assert.False(t, result.OK(), "expected %q to be invalid", v)
			assert.Contains(t, result.Errors[0].Message, "expected a boolean")
		})
	}
}

func TestValidate_URLType(t *testing.T) {
	tests := []struct {
		name  string
		value string
		ok    bool
	}{
		{"https url", "https://example.com", true},
		{"http url", "http://localhost:8080/path", true},
		{"postgres url", "postgres://user:pass@host:5432/db", true},
		{"no scheme", "example.com", false},
		{"no host", "http://", false},
		{"just text", "not-a-url", false},
	}

	s := &Schema{Keys: map[string]Rule{
		"URL": {Type: "url"},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Validate(map[string]string{"URL": tt.value})
			if tt.ok {
				assert.True(t, result.OK(), "expected %q to be a valid URL", tt.value)
			} else {
				assert.False(t, result.OK(), "expected %q to be invalid", tt.value)
				assert.Contains(t, result.Errors[0].Message, "expected a valid URL")
			}
		})
	}
}

func TestValidate_EnumType(t *testing.T) {
	s := &Schema{Keys: map[string]Rule{
		"LEVEL": {Type: "enum", Values: []string{"debug", "info", "warn", "error"}},
	}}

	t.Run("valid value", func(t *testing.T) {
		result := s.Validate(map[string]string{"LEVEL": "info"})
		assert.True(t, result.OK())
	})

	t.Run("invalid value", func(t *testing.T) {
		result := s.Validate(map[string]string{"LEVEL": "trace"})
		assert.False(t, result.OK())
		assert.Contains(t, result.Errors[0].Message, "expected one of")
		assert.Contains(t, result.Errors[0].Message, "trace")
	})

	t.Run("case sensitive", func(t *testing.T) {
		result := s.Validate(map[string]string{"LEVEL": "DEBUG"})
		assert.False(t, result.OK())
	})
}

func TestValidate_EmailType(t *testing.T) {
	tests := []struct {
		name  string
		value string
		ok    bool
	}{
		{"valid email", "user@example.com", true},
		{"subdomain email", "admin@sub.example.co.uk", true},
		{"no at sign", "userexample.com", false},
		{"no domain", "user@", false},
		{"no local", "@example.com", false},
		{"no dot in domain", "user@localhost", false},
	}

	s := &Schema{Keys: map[string]Rule{
		"EMAIL": {Type: "email"},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Validate(map[string]string{"EMAIL": tt.value})
			if tt.ok {
				assert.True(t, result.OK(), "expected %q to be a valid email", tt.value)
			} else {
				assert.False(t, result.OK(), "expected %q to be invalid", tt.value)
				assert.Contains(t, result.Errors[0].Message, "expected a valid email")
			}
		})
	}
}

func TestValidate_PortType(t *testing.T) {
	tests := []struct {
		name  string
		value string
		ok    bool
	}{
		{"port 80", "80", true},
		{"port 443", "443", true},
		{"port 1", "1", true},
		{"port 65535", "65535", true},
		{"port 0", "0", false},
		{"port 65536", "65536", false},
		{"negative port", "-1", false},
		{"not a number", "abc", false},
		{"float", "80.5", false},
	}

	s := &Schema{Keys: map[string]Rule{
		"PORT": {Type: "port"},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Validate(map[string]string{"PORT": tt.value})
			if tt.ok {
				assert.True(t, result.OK(), "expected %q to be a valid port", tt.value)
			} else {
				assert.False(t, result.OK(), "expected %q to be invalid", tt.value)
				assert.Contains(t, result.Errors[0].Message, "expected a port number")
			}
		})
	}
}

func TestValidate_Pattern(t *testing.T) {
	s := &Schema{Keys: map[string]Rule{
		"API_KEY": {Type: "string", Pattern: `^sk-[a-zA-Z0-9]{20,}$`},
	}}

	t.Run("matches pattern", func(t *testing.T) {
		result := s.Validate(map[string]string{"API_KEY": "sk-abcdefghij1234567890"})
		assert.True(t, result.OK())
	})

	t.Run("does not match pattern", func(t *testing.T) {
		result := s.Validate(map[string]string{"API_KEY": "bad-key"})
		assert.False(t, result.OK())
		assert.Contains(t, result.Errors[0].Message, "does not match pattern")
	})
}

func TestValidate_MultipleErrors(t *testing.T) {
	s := &Schema{Keys: map[string]Rule{
		"DB_HOST": {Type: "string", Required: true},
		"DB_PORT": {Type: "port", Required: true},
		"DEBUG":   {Type: "boolean"},
	}}

	result := s.Validate(map[string]string{
		"DB_PORT": "not-a-port",
		"DEBUG":   "maybe",
	})
	assert.False(t, result.OK())
	// DB_HOST required but missing, DB_PORT invalid type, DEBUG invalid type.
	assert.Len(t, result.Errors, 3)
}

func TestValidate_ExtraKeysIgnored(t *testing.T) {
	s := &Schema{Keys: map[string]Rule{
		"DB_HOST": {Type: "string"},
	}}

	// EXTRA_KEY is not in the schema; it should be silently ignored.
	result := s.Validate(map[string]string{
		"DB_HOST":   "localhost",
		"EXTRA_KEY": "extra-value",
	})
	assert.True(t, result.OK())
}

func TestValidate_PatternAndType(t *testing.T) {
	// When both type and pattern are specified, both must pass.
	s := &Schema{Keys: map[string]Rule{
		"PORT": {Type: "port", Pattern: `^80\d{2}$`},
	}}

	t.Run("valid port and pattern", func(t *testing.T) {
		result := s.Validate(map[string]string{"PORT": "8080"})
		assert.True(t, result.OK())
	})

	t.Run("valid port but bad pattern", func(t *testing.T) {
		result := s.Validate(map[string]string{"PORT": "3000"})
		assert.False(t, result.OK())
		assert.Contains(t, result.Errors[0].Message, "does not match pattern")
	})

	t.Run("bad type", func(t *testing.T) {
		result := s.Validate(map[string]string{"PORT": "abc"})
		assert.False(t, result.OK())
		assert.Contains(t, result.Errors[0].Message, "expected a port number")
	})
}

func TestValidationError_String(t *testing.T) {
	e := ValidationError{Key: "DB_HOST", Message: "required key is missing"}
	assert.Equal(t, "DB_HOST: required key is missing", e.String())
}

func TestResult_OK(t *testing.T) {
	t.Run("empty errors is OK", func(t *testing.T) {
		r := &Result{}
		assert.True(t, r.OK())
	})

	t.Run("with errors is not OK", func(t *testing.T) {
		r := &Result{Errors: []ValidationError{{Key: "X", Message: "bad"}}}
		assert.False(t, r.OK())
	})
}
