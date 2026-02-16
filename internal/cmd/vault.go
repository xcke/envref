package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/backend"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/output"
	"golang.org/x/term"
)

const defaultVaultExportFile = "envref-vault-export.json"

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
	cmd.AddCommand(newVaultLockCmd())
	cmd.AddCommand(newVaultUnlockCmd())
	cmd.AddCommand(newVaultExportCmd())
	cmd.AddCommand(newVaultImportCmd())

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

// newVaultLockCmd creates the vault lock subcommand.
func newVaultLockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Lock the vault to prevent secret access",
		Long: `Lock the local encrypted vault to prevent all secret access.

When the vault is locked, all get, set, delete, and list operations will be
refused until the vault is unlocked with 'envref vault unlock'.

Your passphrase is verified before locking to ensure only authorized users
can lock the vault.

Examples:
  envref vault lock                                 # interactive passphrase prompt
  ENVREF_VAULT_PASSPHRASE=secret envref vault lock  # non-interactive`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVaultLock(cmd)
		},
	}

	return cmd
}

// runVaultLock locks the vault after verifying the passphrase.
func runVaultLock(cmd *cobra.Command) error {
	out := output.NewWriter(cmd)

	v, err := createVaultForCommand(cmd)
	if err != nil {
		return err
	}
	defer func() { _ = v.Close() }()

	if err := v.Lock(); err != nil {
		return fmt.Errorf("locking vault: %w", err)
	}

	out.Info("vault locked at %s\n", v.DBPath())
	return nil
}

// newVaultUnlockCmd creates the vault unlock subcommand.
func newVaultUnlockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Unlock the vault to allow secret access",
		Long: `Unlock the local encrypted vault to allow secret access again.

The vault must have been previously locked with 'envref vault lock'. Your
passphrase is verified before unlocking.

Examples:
  envref vault unlock                                 # interactive passphrase prompt
  ENVREF_VAULT_PASSPHRASE=secret envref vault unlock  # non-interactive`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVaultUnlock(cmd)
		},
	}

	return cmd
}

// runVaultUnlock unlocks the vault after verifying the passphrase.
func runVaultUnlock(cmd *cobra.Command) error {
	out := output.NewWriter(cmd)

	v, err := createVaultForCommand(cmd)
	if err != nil {
		return err
	}
	defer func() { _ = v.Close() }()

	if err := v.Unlock(); err != nil {
		return fmt.Errorf("unlocking vault: %w", err)
	}

	out.Info("vault unlocked at %s\n", v.DBPath())
	return nil
}

// newVaultExportCmd creates the vault export subcommand.
func newVaultExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export vault secrets to a JSON file",
		Long: `Export all secrets from the local encrypted vault to a JSON file.

The exported file contains decrypted secret key-value pairs in plaintext JSON.
Keep this file secure — it contains all your vault secrets in the clear.

By default, exports to envref-vault-export.json in the current directory. Use
--file to specify a different path, or --stdout to write to standard output.

Examples:
  envref vault export                                  # export to envref-vault-export.json
  envref vault export --file backup.json               # export to custom file
  envref vault export --stdout                         # export to stdout (for piping)
  envref vault export --stdout | age -r <recipient>    # export and encrypt with age`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVaultExport(cmd)
		},
	}

	cmd.Flags().StringP("file", "f", "", "output file path (default: "+defaultVaultExportFile+")")
	cmd.Flags().Bool("stdout", false, "write export to stdout instead of a file")

	return cmd
}

// runVaultExport exports all vault secrets to JSON.
func runVaultExport(cmd *cobra.Command) error {
	out := output.NewWriter(cmd)

	v, err := createVaultForCommand(cmd)
	if err != nil {
		return err
	}
	defer func() { _ = v.Close() }()

	data, err := v.ExportJSON()
	if err != nil {
		return fmt.Errorf("exporting vault: %w", err)
	}

	toStdout, _ := cmd.Flags().GetBool("stdout")
	if toStdout {
		_, err := fmt.Fprintln(out.Stdout(), string(data))
		return err
	}

	filePath, _ := cmd.Flags().GetString("file")
	if filePath == "" {
		filePath = defaultVaultExportFile
	}

	if err := os.WriteFile(filePath, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("writing export file: %w", err)
	}

	export, _ := v.Export()
	out.Info("exported %d secrets to %s\n", len(export.Secrets), filePath)
	out.Warn("this file contains plaintext secrets — keep it secure and delete after use\n")
	return nil
}

// newVaultImportCmd creates the vault import subcommand.
func newVaultImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import secrets into the vault from a JSON file",
		Long: `Import secrets from a JSON export file into the local encrypted vault.

The file must be in the format produced by 'envref vault export'. Each secret
is re-encrypted with the current vault passphrase on import.

Existing keys in the vault are overwritten by imported values.

By default, reads from envref-vault-export.json in the current directory. Use
--file to specify a different path, or --stdin to read from standard input.

Examples:
  envref vault import                                  # import from envref-vault-export.json
  envref vault import --file backup.json               # import from custom file
  cat backup.json | envref vault import --stdin         # import from stdin`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVaultImport(cmd)
		},
	}

	cmd.Flags().StringP("file", "f", "", "input file path (default: "+defaultVaultExportFile+")")
	cmd.Flags().Bool("stdin", false, "read import data from stdin instead of a file")

	return cmd
}

// runVaultImport imports secrets from a JSON file into the vault.
func runVaultImport(cmd *cobra.Command) error {
	out := output.NewWriter(cmd)

	var data []byte
	var err error
	var source string

	fromStdin, _ := cmd.Flags().GetBool("stdin")
	if fromStdin {
		data, err = readAll(cmd)
		if err != nil {
			return fmt.Errorf("reading from stdin: %w", err)
		}
		source = "stdin"
	} else {
		filePath, _ := cmd.Flags().GetString("file")
		if filePath == "" {
			filePath = defaultVaultExportFile
		}
		data, err = os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("reading import file: %w", err)
		}
		source = filePath
	}

	v, err := createVaultForCommand(cmd)
	if err != nil {
		return err
	}
	defer func() { _ = v.Close() }()

	count, err := v.ImportJSON(data)
	if err != nil {
		return fmt.Errorf("importing vault: %w", err)
	}

	out.Info("imported %d secrets from %s\n", count, source)
	return nil
}

// readAll reads all data from the command's stdin.
func readAll(cmd *cobra.Command) ([]byte, error) {
	return io.ReadAll(cmd.InOrStdin())
}

// createVaultForCommand creates a VaultBackend for vault management commands
// (lock, unlock). It loads config, resolves the passphrase, and creates the
// backend without verifying the passphrase (lock/unlock verify internally).
func createVaultForCommand(cmd *cobra.Command) (*backend.VaultBackend, error) {
	var bc config.BackendConfig
	cwd, err := os.Getwd()
	if err == nil {
		cfg, _, loadErr := config.Load(cwd)
		if loadErr == nil {
			bc = findVaultBackendConfig(cfg)
		}
	}

	passphrase := os.Getenv("ENVREF_VAULT_PASSPHRASE")
	if passphrase == "" {
		passphrase = bc.Config["passphrase"]
	}
	if passphrase == "" {
		prompted, promptErr := promptVaultPassphraseForAccess(cmd)
		if promptErr != nil {
			return nil, promptErr
		}
		passphrase = prompted
	}

	var opts []backend.VaultOption
	if path := bc.Config["path"]; path != "" {
		opts = append(opts, backend.WithVaultPath(path))
	}

	v, err := backend.NewVaultBackend(passphrase, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating vault: %w", err)
	}

	return v, nil
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

