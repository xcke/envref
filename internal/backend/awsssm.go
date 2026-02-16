// Package backend provides the AWS SSM Parameter Store backend, which
// delegates secret operations to the AWS CLI (`aws ssm` subcommands).
//
// # Prerequisites
//
// The AWS CLI v2 must be installed and configured with valid credentials:
//
//	brew install awscli        # or see https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html
//	aws configure              # or use environment variables / IAM roles
//
// # Configuration
//
// In .envref.yaml:
//
//	backends:
//	  - name: ssm
//	    type: aws-ssm
//	    config:
//	      prefix: /myapp/prod    # parameter name prefix (default: "/envref")
//	      region: us-east-1      # AWS region (optional, uses CLI default)
//	      profile: myprofile     # AWS CLI named profile (optional)
//
// # How secrets are stored
//
// Secrets are stored as SSM SecureString parameters. The parameter name
// is constructed as "<prefix>/<key>" (e.g., "/envref/api_key"). The
// secret value is the parameter's decrypted value.
package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Default timeout for AWS CLI operations.
const awsSSMTimeout = 30 * time.Second

// AWSSSMBackend stores secrets in AWS Systems Manager Parameter Store
// via the `aws` CLI. Each secret is a SecureString parameter whose name
// is prefixed with a configurable path prefix.
type AWSSSMBackend struct {
	prefix  string        // parameter name prefix (e.g., "/envref")
	region  string        // optional AWS region
	profile string        // optional AWS CLI named profile
	command string        // path to the aws CLI executable
	timeout time.Duration // max time per CLI invocation
}

// AWSSSMOption configures optional settings for AWSSSMBackend.
type AWSSSMOption func(*AWSSSMBackend)

// WithAWSSSMRegion sets the AWS region for SSM operations.
func WithAWSSSMRegion(region string) AWSSSMOption {
	return func(b *AWSSSMBackend) {
		b.region = region
	}
}

// WithAWSSSMProfile sets the AWS CLI named profile.
func WithAWSSSMProfile(profile string) AWSSSMOption {
	return func(b *AWSSSMBackend) {
		b.profile = profile
	}
}

// WithAWSSSMCommand overrides the path to the aws CLI executable.
func WithAWSSSMCommand(command string) AWSSSMOption {
	return func(b *AWSSSMBackend) {
		b.command = command
	}
}

// NewAWSSSMBackend creates a new AWSSSMBackend that delegates to the `aws` CLI.
// The prefix parameter specifies the SSM parameter name prefix.
func NewAWSSSMBackend(prefix string, opts ...AWSSSMOption) *AWSSSMBackend {
	b := &AWSSSMBackend{
		prefix:  prefix,
		command: "aws",
		timeout: awsSSMTimeout,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Name returns "aws-ssm", the identifier used in .envref.yaml configuration
// and ref:// URIs.
func (b *AWSSSMBackend) Name() string {
	return "aws-ssm"
}

// ssmParameter represents the relevant fields from `aws ssm get-parameter`.
type ssmParameter struct {
	Parameter struct {
		Name  string `json:"Name"`
		Value string `json:"Value"`
	} `json:"Parameter"`
}

// ssmParameterList represents the response from `aws ssm describe-parameters`.
type ssmParameterList struct {
	Parameters []struct {
		Name string `json:"Name"`
	} `json:"Parameters"`
	NextToken *string `json:"NextToken,omitempty"`
}

// paramName returns the full SSM parameter name for a given key.
func (b *AWSSSMBackend) paramName(key string) string {
	return b.prefix + "/" + key
}

// Get retrieves the secret value for the given key from AWS SSM Parameter Store.
// Returns ErrNotFound if no parameter with that name exists.
func (b *AWSSSMBackend) Get(key string) (string, error) {
	args := []string{
		"ssm", "get-parameter",
		"--name", b.paramName(key),
		"--with-decryption",
		"--output", "json",
	}
	args = b.appendGlobalFlags(args)

	stdout, err := b.run(args)
	if err != nil {
		if isAWSNotFoundErr(err) {
			return "", ErrNotFound
		}
		return "", NewKeyError(b.Name(), key, fmt.Errorf("aws ssm get-parameter: %w", err))
	}

	var result ssmParameter
	if err := json.Unmarshal(stdout, &result); err != nil {
		return "", NewKeyError(b.Name(), key, fmt.Errorf("parse response: %w", err))
	}

	return result.Parameter.Value, nil
}

// Set stores a secret value under the given key in AWS SSM Parameter Store.
// If a parameter with that name already exists, it is overwritten.
func (b *AWSSSMBackend) Set(key, value string) error {
	args := []string{
		"ssm", "put-parameter",
		"--name", b.paramName(key),
		"--value", value,
		"--type", "SecureString",
		"--overwrite",
		"--output", "json",
	}
	args = b.appendGlobalFlags(args)

	if _, err := b.run(args); err != nil {
		return NewKeyError(b.Name(), key, fmt.Errorf("aws ssm put-parameter: %w", err))
	}
	return nil
}

// Delete removes the secret for the given key from AWS SSM Parameter Store.
// Returns ErrNotFound if no parameter with that name exists.
func (b *AWSSSMBackend) Delete(key string) error {
	args := []string{
		"ssm", "delete-parameter",
		"--name", b.paramName(key),
		"--output", "json",
	}
	args = b.appendGlobalFlags(args)

	if _, err := b.run(args); err != nil {
		if isAWSNotFoundErr(err) {
			return ErrNotFound
		}
		return NewKeyError(b.Name(), key, fmt.Errorf("aws ssm delete-parameter: %w", err))
	}
	return nil
}

// List returns all secret keys (parameter names) under the configured prefix.
// The prefix is stripped from the returned keys.
func (b *AWSSSMBackend) List() ([]string, error) {
	var allKeys []string
	var nextToken *string

	for {
		args := []string{
			"ssm", "describe-parameters",
			"--parameter-filters",
			fmt.Sprintf("Key=Name,Option=BeginsWith,Values=%s/", b.prefix),
			"--output", "json",
		}
		if nextToken != nil {
			args = append(args, "--next-token", *nextToken)
		}
		args = b.appendGlobalFlags(args)

		stdout, err := b.run(args)
		if err != nil {
			return nil, fmt.Errorf("aws-ssm list: %w", err)
		}

		var result ssmParameterList
		if err := json.Unmarshal(stdout, &result); err != nil {
			return nil, fmt.Errorf("aws-ssm list: parse response: %w", err)
		}

		prefixWithSlash := b.prefix + "/"
		for _, p := range result.Parameters {
			key := strings.TrimPrefix(p.Name, prefixWithSlash)
			allKeys = append(allKeys, key)
		}

		if result.NextToken == nil || *result.NextToken == "" {
			break
		}
		nextToken = result.NextToken
	}

	if allKeys == nil {
		return []string{}, nil
	}
	return allKeys, nil
}

// appendGlobalFlags adds --region and --profile flags if configured.
func (b *AWSSSMBackend) appendGlobalFlags(args []string) []string {
	if b.region != "" {
		args = append(args, "--region", b.region)
	}
	if b.profile != "" {
		args = append(args, "--profile", b.profile)
	}
	return args
}

// run executes the aws CLI with the given arguments and returns stdout.
func (b *AWSSSMBackend) run(args []string) ([]byte, error) {
	cmd := exec.Command(b.command, args...) //nolint:gosec // Command path comes from trusted config or default "aws"

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start aws: %w", err)
	}

	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			stderrMsg := strings.TrimSpace(stderr.String())
			if stderrMsg != "" {
				return nil, fmt.Errorf("%s", stderrMsg)
			}
			return nil, err
		}
	case <-time.After(b.timeout):
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("aws cli timed out after %s", b.timeout)
	}

	return stdout.Bytes(), nil
}

// isAWSNotFoundErr checks whether an error from the AWS CLI indicates that
// a parameter was not found. The AWS CLI outputs error messages containing
// "ParameterNotFound" or "ParameterNotFoundException".
func isAWSNotFoundErr(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "parameternotfound") ||
		strings.Contains(msg, "parameter not found") ||
		strings.Contains(msg, "does not exist")
}
