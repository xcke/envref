package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/backend"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/output"
	"golang.org/x/term"
)

// newVaultCmd creates the vault command group for managing the encrypted vault.
func newVaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Manage the local encrypted vault",
		Long: `Manage the local encrypted vault backend used to store secrets.

The vault stores secrets in a SQLite database with per-value age encryption.
Use 'vault init' to set up the vault with a master passphrase on first use.`,
	}

	cmd.AddCommand(newVaultInitCmd())

	return cmd
}

// newVaultInitCmd creates the vault init subcommand.
func newVaultInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the vault with a master passphrase",
		Long: `Initialize the local encrypted vault by setting a master passphrase.

The passphrase is used to encrypt and decrypt all secret values stored in the
vault. You will be prompted to enter and confirm the passphrase interactively.

Alternatively, set the ENVREF_VAULT_PASSPHRASE environment variable to
initialize non-interactively (e.g., in CI or scripting).

The vault database is created at ~/.config/envref/vault.db by default, or at
the path configured in .envref.yaml.

Examples:
  envref vault init                                 # interactive passphrase prompt
  ENVREF_VAULT_PASSPHRASE=secret envref vault init  # non-interactive`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVaultInit(cmd)
		},
	}

	return cmd
}

// runVaultInit initializes the vault with a master passphrase.
func runVaultInit(cmd *cobra.Command) error {
	out := output.NewWriter(cmd)

	// Try to load project config for custom vault path; fall back to defaults.
	var bc config.BackendConfig
	cwd, err := os.Getwd()
	if err == nil {
		cfg, _, loadErr := config.Load(cwd)
		if loadErr == nil {
			bc = findVaultBackendConfig(cfg)
		}
	}

	// Resolve passphrase: env var first, then interactive prompt.
	passphrase := os.Getenv("ENVREF_VAULT_PASSPHRASE")
	if passphrase == "" {
		passphrase = bc.Config["passphrase"]
	}
	if passphrase == "" {
		prompted, promptErr := promptVaultPassphrase(cmd, true)
		if promptErr != nil {
			return promptErr
		}
		passphrase = prompted
	}

	// Create the vault backend with the passphrase.
	var opts []backend.VaultOption
	if path := bc.Config["path"]; path != "" {
		opts = append(opts, backend.WithVaultPath(path))
	}

	v, err := backend.NewVaultBackend(passphrase, opts...)
	if err != nil {
		return fmt.Errorf("creating vault: %w", err)
	}
	defer func() { _ = v.Close() }()

	// Check if already initialized.
	initialized, err := v.IsInitialized()
	if err != nil {
		return fmt.Errorf("checking vault state: %w", err)
	}
	if initialized {
		out.Info("vault is already initialized at %s\n", v.DBPath())
		return nil
	}

	// Initialize the vault with a verification token.
	if err := v.Initialize(); err != nil {
		return fmt.Errorf("initializing vault: %w", err)
	}

	out.Info("vault initialized at %s\n", v.DBPath())
	out.Verbose("passphrase verification token stored\n")
	return nil
}

// promptVaultPassphrase prompts the user to enter a vault passphrase from
// the terminal. If confirm is true, the user is asked to enter the passphrase
// twice for confirmation.
func promptVaultPassphrase(cmd *cobra.Command, confirm bool) (string, error) {
	stderr := cmd.ErrOrStderr()

	// Check if stdin is a terminal for secure input.
	stdinFd, isTerm := getTerminalFd(cmd)
	if !isTerm {
		return "", fmt.Errorf("vault passphrase required: set ENVREF_VAULT_PASSPHRASE or use an interactive terminal")
	}

	_, _ = fmt.Fprint(stderr, "Enter vault passphrase: ")
	passBytes, err := term.ReadPassword(stdinFd)
	_, _ = fmt.Fprintln(stderr)
	if err != nil {
		return "", fmt.Errorf("reading passphrase: %w", err)
	}

	passphrase := string(passBytes)
	if passphrase == "" {
		return "", fmt.Errorf("passphrase must not be empty")
	}

	if confirm {
		_, _ = fmt.Fprint(stderr, "Confirm vault passphrase: ")
		confirmBytes, err := term.ReadPassword(stdinFd)
		_, _ = fmt.Fprintln(stderr)
		if err != nil {
			return "", fmt.Errorf("reading passphrase confirmation: %w", err)
		}

		if passphrase != string(confirmBytes) {
			return "", fmt.Errorf("passphrases do not match")
		}
	}

	return passphrase, nil
}

// getTerminalFd returns the file descriptor for stdin and whether it is a
// terminal. It checks both the cobra command's InOrStdin and os.Stdin.
func getTerminalFd(cmd *cobra.Command) (int, bool) {
	if f, ok := cmd.InOrStdin().(*os.File); ok {
		fd := int(f.Fd())
		return fd, term.IsTerminal(fd)
	}
	// Fallback to os.Stdin.
	fd := int(os.Stdin.Fd())
	return fd, term.IsTerminal(fd)
}

// findVaultBackendConfig returns the BackendConfig for the vault backend
// from the config, or a zero-value BackendConfig if none is found.
func findVaultBackendConfig(cfg *config.Config) config.BackendConfig {
	if cfg == nil {
		return config.BackendConfig{}
	}
	for _, bc := range cfg.Backends {
		if bc.EffectiveType() == "vault" {
			return bc
		}
	}
	return config.BackendConfig{}
}

// promptVaultPassphraseForAccess prompts for the vault passphrase (without
// confirmation) when accessing an existing vault. Returns the passphrase
// entered by the user.
func promptVaultPassphraseForAccess(cmd *cobra.Command) (string, error) {
	return promptVaultPassphrase(cmd, false)
}

// createVaultBackendInteractive creates a VaultBackend, prompting for the
// passphrase interactively if not provided via env var or config.
// The cmd parameter is used for terminal I/O; pass nil to disable interactive
// prompting.
func createVaultBackendInteractive(bc config.BackendConfig, cmd *cobra.Command) (*backend.VaultBackend, error) {
	// Resolve passphrase: env var > config > interactive prompt.
	passphrase := os.Getenv("ENVREF_VAULT_PASSPHRASE")
	if passphrase == "" {
		passphrase = bc.Config["passphrase"]
	}
	if passphrase == "" && cmd != nil {
		prompted, err := promptVaultPassphraseForAccess(cmd)
		if err != nil {
			return nil, fmt.Errorf("vault passphrase: %w", err)
		}
		passphrase = prompted
	}
	if passphrase == "" {
		return nil, fmt.Errorf("vault passphrase required: set ENVREF_VAULT_PASSPHRASE or config.passphrase in %s", config.FullFileName)
	}

	var opts []backend.VaultOption
	if path := bc.Config["path"]; path != "" {
		opts = append(opts, backend.WithVaultPath(path))
	}

	v, err := backend.NewVaultBackend(passphrase, opts...)
	if err != nil {
		return nil, err
	}

	// If the vault is initialized, verify the passphrase against the stored token.
	initialized, checkErr := v.IsInitialized()
	if checkErr != nil {
		_ = v.Close()
		return nil, fmt.Errorf("checking vault: %w", checkErr)
	}
	if initialized {
		if verifyErr := v.VerifyPassphrase(); verifyErr != nil {
			_ = v.Close()
			return nil, verifyErr
		}
	}

	return v, nil
}

// vaultCmdContext holds the cobra command for passing to backend creation
// when interactive prompting is needed.
var vaultCmdContext *cobra.Command

// setVaultCmdContext stores the cobra command for use during vault backend
// creation. This allows the backend factory to prompt interactively.
func setVaultCmdContext(cmd *cobra.Command) {
	vaultCmdContext = cmd
}

// clearVaultCmdContext removes the stored cobra command context.
func clearVaultCmdContext() {
	vaultCmdContext = nil
}

// createVaultBackendWithContext creates a VaultBackend using the global
// command context for interactive prompting if available.
func createVaultBackendWithContext(bc config.BackendConfig) (*backend.VaultBackend, error) {
	return createVaultBackendInteractive(bc, vaultCmdContext)
}

