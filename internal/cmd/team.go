package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/output"
)

// newTeamCmd creates the team command group for managing team members.
func newTeamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Manage team members and their public keys",
		Long: `Manage team members and their age public keys for secret sharing.

Team members are stored in the .envref.yaml config file under the "team"
section. Each member has a name (identifier) and an age X25519 public key
used for encrypting secrets during sync push.

Example .envref.yaml team section:
  team:
    - name: alice
      public_key: age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
    - name: bob
      public_key: age1...`,
	}

	cmd.AddCommand(newTeamListCmd())
	cmd.AddCommand(newTeamAddCmd())
	cmd.AddCommand(newTeamRemoveCmd())

	return cmd
}

// newTeamListCmd creates the team list subcommand.
func newTeamListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List team members and their public keys",
		Long: `List all team members defined in the project's .envref.yaml config.

Shows each member's name and age public key. These keys are used as
recipients when running "envref sync push --to-team".

Examples:
  envref team list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamList(cmd)
		},
	}
}

// newTeamAddCmd creates the team add subcommand.
func newTeamAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <public-key>",
		Short: "Add a team member with their age public key",
		Long: `Add a new team member to the project's .envref.yaml config.

The name is an identifier for the member (e.g., "alice", "bob").
The public key must be a valid age X25519 public key (age1...).

Examples:
  envref team add alice age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
  envref team add bob age1...`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamAdd(cmd, args[0], args[1])
		},
	}
}

// newTeamRemoveCmd creates the team remove subcommand.
func newTeamRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a team member from the config",
		Long: `Remove a team member from the project's .envref.yaml config.

The member's entry (name and public key) will be removed from the team
section. If the team section becomes empty, it will be removed entirely.

Examples:
  envref team remove alice`,
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"rm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamRemove(cmd, args[0])
		},
	}
}

// runTeamList prints all team members and their public keys.
func runTeamList(cmd *cobra.Command) error {
	w := output.NewWriter(cmd)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, _, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(cfg.Team) == 0 {
		w.Info("no team members configured\n")
		w.Info("add members with: envref team add <name> <age-public-key>\n")
		return nil
	}

	// Sort by name for consistent output.
	members := make([]config.TeamMember, len(cfg.Team))
	copy(members, cfg.Team)
	sort.Slice(members, func(i, j int) bool {
		return members[i].Name < members[j].Name
	})

	for _, m := range members {
		w.Info("%-20s %s\n", m.Name, m.PublicKey)
	}

	w.Verbose("\n%d team member(s) configured\n", len(members))
	return nil
}

// runTeamAdd adds a new team member to the config file.
func runTeamAdd(cmd *cobra.Command, name, publicKey string) error {
	w := output.NewWriter(cmd)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	_, configDir, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	configPath := filepath.Join(configDir, config.FullFileName)

	if err := config.AddTeamMember(configPath, name, publicKey); err != nil {
		return err
	}

	// Validate the updated config to catch invalid keys early.
	updatedCfg, err := config.LoadFile(configPath)
	if err != nil {
		return fmt.Errorf("reloading config: %w", err)
	}
	if err := updatedCfg.Validate(); err != nil {
		// Rollback would be complex; just report the validation error.
		return fmt.Errorf("config validation failed after adding member: %w", err)
	}

	w.Info("added team member %q with public key %s\n", name, truncateKey(publicKey))
	return nil
}

// runTeamRemove removes a team member from the config file.
func runTeamRemove(cmd *cobra.Command, name string) error {
	w := output.NewWriter(cmd)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	_, configDir, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	configPath := filepath.Join(configDir, config.FullFileName)

	if err := config.RemoveTeamMember(configPath, name); err != nil {
		return err
	}

	w.Info("removed team member %q\n", name)
	return nil
}
