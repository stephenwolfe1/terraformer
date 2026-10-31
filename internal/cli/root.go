// Package cli implements the terraformer command line.
package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/stephenwolfe1/terraformer/internal/terraform"
)

// setting describes one configuration value: its flag, its environment
// variables (first match wins) and default. Every setting can also be given
// as TERRAFORMER_<FLAG_NAME>.
type setting struct {
	flag    string
	short   string
	envs    []string
	def     any
	usage   string
	boolean bool
}

var settings = []setting{
	// Deployment context (all optional).
	{flag: "repo", envs: []string{"REPO"}, def: "", usage: "repository/project name for context (env REPO)"},
	{flag: "account", envs: []string{"ACCOUNT"}, def: "", usage: "account/subscription/project for context (env ACCOUNT)"},
	{flag: "environment", short: "e", envs: []string{"ENVIRONMENT"}, def: "", usage: "environment for context (env ENVIRONMENT)"},
	{flag: "region", short: "r", envs: []string{"REGION"}, def: "", usage: "region for context (env REGION)"},

	// Target selection.
	{flag: "services", envs: []string{"SERVICES"}, def: "", usage: "space-separated service names under --services-dir to run when no path is given (env SERVICES)"},
	{flag: "services-dir", envs: []string{"SERVICES_DIR"}, def: "services", usage: "directory containing services (env SERVICES_DIR)"},
	{flag: "directory", short: "d", envs: []string{"DIRECTORY"}, def: "", usage: "default directory when no path or services are given (env DIRECTORY)"},

	// Service gating.
	{flag: "state-file", envs: nil, def: "state.yaml", usage: "per-service file that enables a service per account/environment/region; empty disables gating"},
	{flag: "require-state-file", envs: nil, def: false, boolean: true, usage: "skip services that have no state file"},

	// Terraform behavior.
	{flag: "backend-key", envs: nil, def: "", usage: "backend key template passed as -backend-config=key=..., e.g. {repo}/{service}/{account}/{environment}/{region}/terraform.tfstate"},
	{flag: "extra-args", envs: nil, def: "", usage: "extra whitespace-separated flags appended to the terraform subcommand"},
	{flag: "auto-approve", envs: nil, def: false, boolean: true, usage: "apply without prompting (adds -auto-approve -input=false)"},
	{flag: "save-plan", envs: nil, def: false, boolean: true, usage: "plan: save terraform.tfplan and a JSON rendering of it"},
	{flag: "plan-json-file", envs: nil, def: "", usage: "plan: override the plan JSON filename (default <account>-<environment>-<region>-<service>.json)"},
	{flag: "clean", envs: nil, def: false, boolean: true, usage: "remove .terraform data dir, lock file and plan before and after each run"},
	{flag: "upgrade", envs: nil, def: true, boolean: true, usage: "run init with -upgrade so providers resolve to the newest allowed (mirrored) versions; --upgrade=false to honor lock files strictly"},
	{flag: "binary", envs: nil, def: "terraform", usage: "terraform-compatible binary to execute"},

	// Credentials / integrations.
	{flag: "profile", short: "p", envs: nil, def: "", usage: "AWS profile to export as AWS_PROFILE/AWS_DEFAULT_PROFILE"},
	{flag: "vault-addr", envs: nil, def: "", usage: "Vault address to export as VAULT_ADDR"},

	{flag: "no-color", envs: []string{"NO_COLOR"}, def: false, boolean: true, usage: "disable colored wrapper output"},
}

var rootCmd = &cobra.Command{
	Use:   "terraformer",
	Short: "Terraform wrapper for consistent local and CI/CD runs",
	Long: `terraformer wraps terraform so the same command works the same way on a laptop,
in Docker and in CI. It resolves one or more service directories, optionally
scopes the run to an account/environment/region context, runs a silent init,
then runs the requested subcommand with sensible defaults.

Every flag can also be set with an environment variable named
TERRAFORMER_<FLAG> (e.g. TERRAFORMER_AUTO_APPROVE=true).`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if viper.GetBool("no-color") {
			terraform.SetColor(false)
		}
		terraform.Binary = viper.GetString("binary")
	},
}

func init() {
	viper.SetEnvPrefix("TERRAFORMER")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	flags := rootCmd.PersistentFlags()
	for _, s := range settings {
		if s.boolean {
			flags.BoolP(s.flag, s.short, s.def.(bool), s.usage)
		} else {
			flags.StringP(s.flag, s.short, s.def.(string), s.usage)
		}
		_ = viper.BindPFlag(s.flag, flags.Lookup(s.flag))
		envs := append([]string{"TERRAFORMER_" + strings.ToUpper(strings.ReplaceAll(s.flag, "-", "_"))}, s.envs...)
		_ = viper.BindEnv(append([]string{s.flag}, envs...)...)
	}
}

// Execute runs the CLI and returns the process exit code. Terraform's own
// exit code is propagated (e.g. 2 from plan -detailed-exitcode).
func Execute() int {
	err := rootCmd.Execute()
	if err == nil {
		return 0
	}
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
		return exitErr.ExitCode()
	}
	return 1
}

func currentContext() terraform.Context {
	return terraform.Context{
		Repo:        viper.GetString("repo"),
		Account:     viper.GetString("account"),
		Environment: viper.GetString("environment"),
		Region:      viper.GetString("region"),
	}
}

// optionsFor builds terraform options for one target from the resolved config.
func optionsFor(t target, passthrough []string) *terraform.Options {
	extra := strings.Fields(viper.GetString("extra-args"))
	extra = append(extra, passthrough...)
	return &terraform.Options{
		Dir:          t.abs,
		Service:      t.rel,
		Context:      currentContext(),
		AWSProfile:   viper.GetString("profile"),
		VaultAddr:    viper.GetString("vault-addr"),
		BackendKey:   viper.GetString("backend-key"),
		ExtraArgs:    extra,
		AutoApprove:  viper.GetBool("auto-approve"),
		SavePlan:     viper.GetBool("save-plan"),
		PlanJSONFile: viper.GetString("plan-json-file"),
		Clean:        viper.GetBool("clean"),
		Upgrade:      viper.GetBool("upgrade"),
	}
}
