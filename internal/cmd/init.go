package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/config"
)

// newInitCmd creates the init subcommand.
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new envref project",
		Long: `Scaffold a new envref project in the current directory.

Creates the following files:
  .envref.yaml   — project configuration (project name, defaults)
  .env           — environment variables with example entries
  .env.local     — local overrides (gitignored)

Optional:
  .envrc         — direnv integration (with --direnv flag)

Existing files are skipped unless --force is used.
The .env.local entry is appended to .gitignore if not already present.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			direnv, _ := cmd.Flags().GetBool("direnv")
			force, _ := cmd.Flags().GetBool("force")
			dir, _ := cmd.Flags().GetString("dir")

			if dir == "" {
				var err error
				dir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("getting working directory: %w", err)
				}
			}

			return runInit(cmd, dir, project, direnv, force)
		},
	}

	cmd.Flags().StringP("project", "p", "", "project name (defaults to current directory name)")
	cmd.Flags().Bool("direnv", false, "generate .envrc for direnv integration")
	cmd.Flags().Bool("force", false, "overwrite existing files")
	cmd.Flags().String("dir", "", "target directory (defaults to current directory)")

	return cmd
}

// runInit scaffolds the envref project files in the given directory.
func runInit(cmd *cobra.Command, dir, project string, direnv, force bool) error {
	out := cmd.OutOrStdout()

	// Default project name to directory basename.
	if project == "" {
		project = filepath.Base(dir)
	}

	// Generate .envref.yaml content.
	configContent := fmt.Sprintf(`# envref project configuration
# See: https://github.com/xcke/envref

project: %s
env_file: .env
local_file: .env.local

# Secret backends (tried in order)
# backends:
#   - name: keychain
#     type: keychain
#   - name: vault
#     type: encrypted-vault

# Environment profiles
# profiles:
#   staging:
#     env_file: .env.staging
#   production:
#     env_file: .env.production
`, project)

	envContent := `# Environment variables for this project
# Secret values should use ref:// references instead of plaintext:
#   API_KEY=ref://secrets/api_key

APP_NAME=myapp
APP_ENV=development
APP_PORT=3000
`

	envLocalContent := `# Local overrides (not committed to git)
# Add personal settings or secret values here
`

	envrcContent := `# Load environment variables via envref
# Requires: direnv (https://direnv.net)
# Run 'direnv allow' after creating this file.

eval "$(envref resolve --direnv 2>/dev/null)" || true
`

	// Write files.
	if err := writeInitFile(out, filepath.Join(dir, config.FullFileName), configContent, force); err != nil {
		return err
	}

	if err := writeInitFile(out, filepath.Join(dir, ".env"), envContent, force); err != nil {
		return err
	}

	if err := writeInitFile(out, filepath.Join(dir, ".env.local"), envLocalContent, force); err != nil {
		return err
	}

	if direnv {
		if err := writeInitFile(out, filepath.Join(dir, ".envrc"), envrcContent, force); err != nil {
			return err
		}
	}

	// Update .gitignore.
	if err := ensureGitignoreEntry(out, filepath.Join(dir, ".gitignore"), ".env.local"); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "\nInitialized envref project %q in %s\n", project, dir)

	if direnv {
		_, _ = fmt.Fprintln(out, "Run 'direnv allow' to activate the .envrc file.")
	}

	return nil
}

// writeInitFile writes content to path. If the file already exists and force
// is false, it prints a skip message and returns nil.
func writeInitFile(out io.Writer, path, content string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			_, _ = fmt.Fprintf(out, "  skip %s (already exists)\n", filepath.Base(path))
			return nil
		}
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	_, _ = fmt.Fprintf(out, "  create %s\n", filepath.Base(path))
	return nil
}

// ensureGitignoreEntry appends entry to the .gitignore file at path if it is
// not already present. Creates the file if it does not exist.
func ensureGitignoreEntry(out io.Writer, path, entry string) error {
	// Read existing content.
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	content := string(data)

	// Check if entry already present (exact line match).
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == entry {
			_, _ = fmt.Fprintf(out, "  skip .gitignore (%s already listed)\n", entry)
			return nil
		}
	}

	// Append entry.
	var newContent string
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		newContent = content + "\n" + entry + "\n"
	} else {
		newContent = content + entry + "\n"
	}

	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	if len(data) == 0 {
		_, _ = fmt.Fprintf(out, "  create .gitignore\n")
	} else {
		_, _ = fmt.Fprintf(out, "  update .gitignore (added %s)\n", entry)
	}

	return nil
}
