package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/resolve"
)

// exitError wraps an exit code so the caller can propagate it.
type exitError struct {
	code int
}

func (e *exitError) Error() string {
	return fmt.Sprintf("exit status %d", e.code)
}

// newRunCmd creates the run subcommand.
func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [flags] -- <command> [args...]",
		Short: "Run a command with resolved environment variables",
		Long: `Resolve all environment variables (including ref:// secret references)
and execute the given command with those variables injected into its
environment. This is an alternative to direnv for one-off commands.

The resolved environment merges:
  .env ← .env.<profile> ← .env.local

All resolved variables are added to the subprocess environment alongside
the current process environment.

Examples:
  envref run -- node server.js
  envref run -- docker compose up
  envref run --profile staging -- ./deploy.sh
  envref run --strict -- make test`,
		// Cobra's built-in -- handling passes everything after -- as args.
		Args: cobra.MinimumNArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			setVaultCmdContext(cmd)
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			clearVaultCmdContext()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			profile, _ := cmd.Flags().GetString("profile")
			strict, _ := cmd.Flags().GetBool("strict")
			return runRun(cmd, args, profile, strict)
		},
	}

	cmd.Flags().StringP("profile", "P", "", "environment profile to use (e.g., staging, production)")
	cmd.Flags().Bool("strict", false, "fail if any reference cannot be resolved")

	return cmd
}

// runRun implements the run command logic.
func runRun(cmd *cobra.Command, cmdArgs []string, profileOverride string, strict bool) error {
	// Resolve environment variables using the same pipeline as "envref resolve".
	entries, err := resolveEnvEntries(cmd, profileOverride, strict)
	if err != nil {
		return err
	}

	// Build the subprocess environment: inherit current env + overlay resolved vars.
	environ := os.Environ()
	for _, entry := range entries {
		environ = append(environ, entry.Key+"="+entry.Value)
	}

	// Find the executable on PATH.
	binary, err := exec.LookPath(cmdArgs[0])
	if err != nil {
		return fmt.Errorf("command not found: %s", cmdArgs[0])
	}

	// Set up the subprocess.
	child := exec.Command(binary, cmdArgs[1:]...)
	child.Env = environ
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr

	// Forward signals to the child process.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigCh {
			if child.Process != nil {
				_ = child.Process.Signal(sig)
			}
		}
	}()
	defer signal.Stop(sigCh)

	// Run and propagate exit code.
	if err := child.Run(); err != nil {
		var execExitErr *exec.ExitError
		if errors.As(err, &execExitErr) {
			return &exitError{code: execExitErr.ExitCode()}
		}
		return fmt.Errorf("running %s: %w", cmdArgs[0], err)
	}

	return nil
}

// resolveEnvEntries runs the full resolve pipeline and returns resolved entries.
func resolveEnvEntries(cmd *cobra.Command, profileOverride string, strict bool) ([]resolve.Entry, error) {
	// Load project config.
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	cfg, projectDir, err := config.Load(cwd)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	// Resolve file paths relative to the project root.
	envPath := resolveFilePath(projectDir, cfg.EnvFile)
	localPath := resolveFilePath(projectDir, cfg.LocalFile)

	// Determine the active profile.
	var profilePath string
	profile := cfg.EffectiveProfile(profileOverride)
	if profile != "" {
		profilePath = resolveFilePath(projectDir, cfg.ProfileEnvFile(profile))
	}

	// Load and merge env files.
	env, err := loadAndMergeEnv(cmd, envPath, profilePath, localPath)
	if err != nil {
		return nil, err
	}

	// If no refs (including embedded nested refs), convert directly.
	if !env.HasAnyRefs() {
		return envToEntries(env), nil
	}

	// Build the backend registry.
	if len(cfg.Backends) == 0 {
		return nil, fmt.Errorf("ref:// references found but no backends configured in %s", config.FullFileName)
	}

	registry, err := buildRegistry(cfg)
	if err != nil {
		return nil, fmt.Errorf("initializing backends: %w", err)
	}
	defer registry.CloseAll()

	// Resolve references.
	result, err := resolve.Resolve(env, registry, cfg.Project)
	if err != nil {
		return nil, fmt.Errorf("resolving references: %w", err)
	}

	// Report resolution errors to stderr.
	for _, keyErr := range result.Errors {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", keyErr.Error())
	}

	// In strict mode, fail if any reference couldn't be resolved.
	if strict && !result.Resolved() {
		return nil, fmt.Errorf("%d reference(s) could not be resolved (strict mode)", len(result.Errors))
	}

	return result.Entries, nil
}
