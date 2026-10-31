package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/stephenwolfe1/terraformer/internal/terraform"
)

const pathHelp = `
With a path, runs in that directory. Without one, runs for each name in
SERVICES (resolved under SERVICES_DIR), else DIRECTORY, else the current
directory. Terraform flags can be passed after "--".`

// forEachTarget resolves targets, applies service gating and calls fn for
// each target that should run. It stops at the first error.
func forEachTarget(cmd *cobra.Command, args []string, fn func(*terraform.Options) error) error {
	positional, passthrough := splitArgs(cmd, args)
	targets, err := resolveTargets(positional)
	if err != nil {
		return err
	}
	g := gate{stateFile: viper.GetString("state-file"), require: viper.GetBool("require-state-file")}
	var changes error // plan -detailed-exitcode: exit 2 means "changes present", keep going
	for _, t := range targets {
		opts := optionsFor(t, passthrough)
		skip, msg, err := checkServiceDir(t.abs, opts.Context, g)
		if err != nil {
			return err
		}
		if skip {
			terraform.PrintStatus(os.Stderr, msg)
			continue
		}
		if err := fn(opts); err != nil {
			if isChangesPresent(cmd.Name(), opts.ExtraArgs, err) {
				changes = fmt.Errorf("changes present in %s: %w", t.rel, err)
				continue
			}
			return fmt.Errorf("%s failed for %s: %w", cmd.Name(), t.rel, err)
		}
	}
	return changes
}

// isChangesPresent reports whether err is plan's "succeeded with changes"
// exit code (2) under -detailed-exitcode.
func isChangesPresent(subcmd string, args []string, err error) bool {
	var exitErr *exec.ExitError
	if subcmd != "plan" || !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
		return false
	}
	return slices.ContainsFunc(args, func(a string) bool {
		return a == "-detailed-exitcode" || a == "--detailed-exitcode"
	})
}

func subcommand(name, short string) *cobra.Command {
	return &cobra.Command{
		Use:   name + " [path] [-- terraform flags...]",
		Short: short,
		Long:  short + "\n" + pathHelp,
		Args:  pathArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return forEachTarget(cmd, args, func(o *terraform.Options) error {
				return terraform.Run(o, name)
			})
		},
	}
}

var initCmd = &cobra.Command{
	Use:   "init [path] [-- terraform flags...]",
	Short: "Run 'terraform init' (output shown)",
	Long:  "Run 'terraform init' (output shown)\n" + pathHelp,
	Args:  pathArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return forEachTarget(cmd, args, terraform.Init)
	},
}

var stateCmd = &cobra.Command{
	Use:   "state [path]",
	Short: "Run terraform init, then open an interactive shell for manual state work",
	Long: `Runs 'terraform init' for a single service, then starts an interactive bash
shell in its directory with the same environment terraform gets (TF_DATA_DIR,
TF_VAR_*, AWS profile, Vault address), so you can run e.g. 'terraform state list'.
With multiple SERVICES, only the first is used.`,
	Args: pathArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		positional, _ := splitArgs(cmd, args)
		targets, err := resolveTargets(positional)
		if err != nil {
			return err
		}
		if len(targets) > 1 {
			terraform.PrintStatus(os.Stderr, fmt.Sprintf("state supports a single service; using %q and ignoring %d other(s)", targets[0].rel, len(targets)-1))
		}
		opts := optionsFor(targets[0], nil)
		opts.ExtraArgs = nil
		return terraform.Shell(opts)
	},
}

func init() {
	rootCmd.AddCommand(
		initCmd,
		subcommand("validate", "Run 'terraform validate' (init without backend first)"),
		subcommand("plan", "Run 'terraform plan' (init and refresh output hidden)"),
		subcommand("apply", "Run 'terraform apply'"),
		subcommand("destroy", "Run 'terraform destroy'"),
		stateCmd,
	)
}
