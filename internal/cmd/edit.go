package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/output"
)

// newEditCmd creates the edit subcommand.
func newEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit [file]",
		Short: "Open a .env file in your editor",
		Long: `Open a .env file in your default editor ($VISUAL or $EDITOR).

By default, opens the project's .env file. Use flags to open alternative
files such as .env.local, a profile-specific file, or the project config.

The editor is determined by checking, in order:
  1. $VISUAL environment variable
  2. $EDITOR environment variable
  3. Falls back to "vi"

Examples:
  envref edit                          # edit .env
  envref edit --local                  # edit .env.local
  envref edit --profile staging        # edit .env.staging
  envref edit --config                 # edit .envref.yaml
  envref edit .env.production          # edit a specific file`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			useLocal, _ := cmd.Flags().GetBool("local")
			useConfig, _ := cmd.Flags().GetBool("config")
			profile, _ := cmd.Flags().GetString("profile")

			var explicitFile string
			if len(args) == 1 {
				explicitFile = args[0]
			}

			return runEdit(cmd, explicitFile, useLocal, useConfig, profile)
		},
	}

	cmd.Flags().BoolP("local", "l", false, "edit .env.local instead of .env")
	cmd.Flags().BoolP("config", "c", false, "edit the .envref.yaml config file")
	cmd.Flags().StringP("profile", "P", "", "edit the .env.<profile> file for the given profile")
	cmd.MarkFlagsMutuallyExclusive("local", "config", "profile")

	return cmd
}

// runEdit opens the resolved file path in the user's editor.
func runEdit(cmd *cobra.Command, explicitFile string, useLocal, useConfig bool, profile string) error {
	w := output.NewWriter(cmd)

	// If an explicit file path was given, open it directly.
	if explicitFile != "" {
		return openEditor(cmd, w, explicitFile)
	}

	// Load project config to resolve file paths.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, projectDir, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Determine which file to edit.
	var targetFile string
	switch {
	case useConfig:
		targetFile = resolveFilePath(projectDir, config.FullFileName)
	case useLocal:
		targetFile = resolveFilePath(projectDir, cfg.LocalFile)
	case profile != "":
		targetFile = resolveFilePath(projectDir, cfg.ProfileEnvFile(profile))
	default:
		targetFile = resolveFilePath(projectDir, cfg.EnvFile)
	}

	return openEditor(cmd, w, targetFile)
}

// openEditor opens the given file in the user's preferred editor.
func openEditor(cmd *cobra.Command, w *output.Writer, filePath string) error {
	editor := editorCommand()
	w.Verbose("opening %s with %s\n", filePath, editor)

	// Verify the file exists before opening.
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	binary, err := exec.LookPath(editor)
	if err != nil {
		return fmt.Errorf("editor %q not found in PATH: %w", editor, err)
	}

	child := exec.Command(binary, filePath)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr

	if err := child.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	return nil
}

// editorCommand returns the user's preferred editor by checking $VISUAL,
// then $EDITOR, falling back to "vi".
func editorCommand() string {
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	return "vi"
}
