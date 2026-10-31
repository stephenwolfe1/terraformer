// Package terraform builds and executes terraform commands for a single
// service directory.
package terraform

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// Binary is the terraform executable to run. Overridable for tests or for
// OpenTofu-compatible binaries (TERRAFORMER_BINARY).
var Binary = "terraform"

// PlanFile is the name of the saved binary plan written when SavePlan is set.
const PlanFile = "terraform.tfplan"

// Options configures how terraform is executed for one service.
type Options struct {
	Dir     string  // absolute directory passed to -chdir
	Service string  // service path as given by the user (for display, slugs and backend key)
	Context Context // optional deployment context

	AWSProfile string // when set, exported as AWS_PROFILE and AWS_DEFAULT_PROFILE
	VaultAddr  string // when set, exported as VAULT_ADDR

	// BackendKey is a template for -backend-config=key=... on init. Supports
	// {repo}, {service}, {account}, {environment} and {region}. Empty = not passed.
	BackendKey string

	ExtraArgs    []string // appended to the main subcommand (after wrapper defaults, so they win)
	AutoApprove  bool     // apply only: add -auto-approve -input=false
	SavePlan     bool     // plan only: write -out=terraform.tfplan and a JSON rendering of it
	PlanJSONFile string   // override for the plan JSON filename
	Clean        bool     // remove the data dir, lock file and plan file before and after the run
	Upgrade      bool     // init -upgrade: select the newest allowed provider versions (the image mirror), ignoring stale lock files
}

type runSpec struct {
	output      OutputMode
	skipSummary bool
}

// DataDir returns the TF_DATA_DIR the wrapper sets for this service, or ""
// when terraform's default (.terraform) is used. An explicit TF_DATA_DIR in
// the environment always wins and is left untouched.
func (o *Options) DataDir() string {
	if os.Getenv("TF_DATA_DIR") != "" || o.Context.IsZero() {
		return ""
	}
	return ".terraform-" + o.Context.Slug(o.Service)
}

// PlanJSONFilename returns the filename used for the saved plan JSON.
func (o *Options) PlanJSONFilename() string {
	if o.PlanJSONFile != "" {
		return o.PlanJSONFile
	}
	return o.Context.Slug(o.Service) + ".json"
}

func (o *Options) serviceName() string {
	name := filepath.Base(o.Service)
	if name == "." || name == "" || name == "/" {
		name = filepath.Base(o.Dir)
	}
	return name
}

// Init runs `terraform init` with output shown.
func Init(opts *Options) error {
	if opts.Clean {
		CleanCache(opts)
	}
	return run(opts, runSpec{output: OutputNormal}, initArgs(opts, true)...)
}

// Run initializes the service silently and then runs subcmd ("plan", "apply",
// "destroy", "validate", or any other terraform subcommand).
func Run(opts *Options, subcmd string) error {
	if opts.Clean {
		CleanCache(opts)
		defer CleanCache(opts)
	}

	PrintStatus(os.Stderr, "Silently running terraform init...")
	if err := run(opts, runSpec{output: OutputSilent, skipSummary: true}, initArgs(opts, subcmd != "validate")...); err != nil {
		return fmt.Errorf("init failed: %w", err)
	}

	args, mode := subcommandArgs(opts, subcmd)
	if subcmd == "plan" {
		PrintStatus(os.Stderr, "Silently refreshing state...")
	}
	err := run(opts, runSpec{output: mode}, args...)

	if err == nil && subcmd == "plan" && opts.SavePlan {
		if serr := savePlanJSON(opts); serr != nil {
			PrintStatus(os.Stderr, "warning: saving plan JSON failed: "+serr.Error())
		}
	}
	return err
}

// Shell initializes the service and starts an interactive bash shell in its
// directory with the same environment terraform would get.
func Shell(opts *Options) error {
	if err := Init(opts); err != nil {
		return fmt.Errorf("init failed: %w", err)
	}
	PrintStatus(os.Stderr, "Starting shell in "+opts.Dir+" (exit to leave)")
	sh := exec.Command("bash", "-i")
	sh.Stdin, sh.Stdout, sh.Stderr = os.Stdin, os.Stdout, os.Stderr
	sh.Dir = opts.Dir
	sh.Env = buildEnv(opts)
	return sh.Run()
}

func initArgs(opts *Options, withBackend bool) []string {
	args := []string{"init", "-input=false"}
	if opts.Upgrade {
		args = append(args, "-upgrade")
	}
	if !withBackend {
		return append(args, "-backend=false")
	}
	args = append(args, "-reconfigure")
	if opts.BackendKey != "" {
		args = append(args, "-backend-config=key="+opts.Context.Expand(opts.BackendKey, opts.Service))
	}
	return args
}

func subcommandArgs(opts *Options, subcmd string) ([]string, OutputMode) {
	args := []string{subcmd}
	mode := OutputNormal
	switch subcmd {
	case "plan":
		args = append(args, "-input=false", "-lock=false")
		if opts.SavePlan {
			args = append(args, "-out="+PlanFile)
		}
		mode = OutputPlanFilter
	case "apply":
		args = append(args, "-lock=true", "-lock-timeout=0s")
		// -input=false also disables the approval prompt, so only add it
		// together with -auto-approve.
		if opts.AutoApprove {
			args = append(args, "-input=false", "-auto-approve")
		}
	case "destroy":
		args = append(args, "-lock=true", "-lock-timeout=0s")
	}
	return append(args, opts.ExtraArgs...), mode
}

// run executes terraform -chdir=<dir> args... with stdio wired according to spec.
func run(opts *Options, spec runSpec, args ...string) error {
	tfArgs := append([]string{"-chdir=" + opts.Dir}, args...)
	fullCmd := Binary + " " + strings.Join(tfArgs, " ")
	cmdName := Binary
	if len(args) > 0 {
		cmdName = args[0]
	}
	service := opts.serviceName()

	PrintCmdStart(os.Stderr, cmdName, service, fullCmd)

	cmd := exec.Command(Binary, tfArgs...)
	cmd.Stdin = os.Stdin
	cmd.Env = buildEnv(opts)

	capture := !spec.skipSummary && ciSummaryCapture()
	var stdoutBuf, stderrBuf bytes.Buffer

	// Silent runs hold back both streams (including warnings such as the
	// mirror's "Incomplete lock file information") and show them only on failure.
	silent := spec.output == OutputSilent
	var heldOut, heldErr bytes.Buffer

	var stderr io.Writer = os.Stderr
	if silent {
		stderr = &heldErr
	}
	if capture {
		stderr = io.MultiWriter(stderr, &stderrBuf)
	}
	cmd.Stderr = stderr

	var stdout io.Writer = os.Stdout
	if silent {
		stdout = &heldOut
	}
	if capture {
		stdout = io.MultiWriter(stdout, &stdoutBuf)
	}
	var pf *PlanFilter
	if spec.output == OutputPlanFilter {
		pf = &PlanFilter{Out: stdout}
		stdout = pf
	}
	cmd.Stdout = stdout

	err := runForwardingSignals(cmd)
	if pf != nil {
		pf.Flush()
	}
	if silent && err != nil {
		_, _ = os.Stdout.Write(heldOut.Bytes())
		_, _ = os.Stderr.Write(heldErr.Bytes())
	}
	PrintCmdFinish(os.Stderr, cmdName, service, err)

	if capture {
		if werr := appendGitHubStepSummary(cmdName, service, fullCmd, err, stdoutBuf.String(), stderrBuf.String()); werr != nil {
			PrintStatus(os.Stderr, "warning: writing CI step summary failed: "+werr.Error())
		}
	}
	return err
}

// runForwardingSignals starts cmd and keeps the wrapper alive on SIGINT/SIGTERM
// so terraform can shut down gracefully and release state locks.
//
// When stdin is a terminal, Ctrl-C already delivers SIGINT to terraform
// (same foreground process group), so forwarding it would count as a second
// interrupt and make terraform exit immediately. In that case SIGINT is only
// swallowed; it is forwarded when there is no terminal (CI, docker stop).
func runForwardingSignals(cmd *exec.Cmd) error {
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	if err := cmd.Start(); err != nil {
		return err
	}

	tty := stdinIsTerminal()
	done := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-sigCh:
				if sig == syscall.SIGINT && tty {
					continue
				}
				_ = cmd.Process.Signal(sig)
			case <-done:
				return
			}
		}
	}()

	err := cmd.Wait()
	close(done)
	return err
}

func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// savePlanJSON renders the saved plan with `terraform show -json` into the
// service directory.
func savePlanJSON(opts *Options) error {
	outFile := opts.PlanJSONFilename()
	PrintStatus(os.Stderr, "Saving plan JSON to "+outFile+"...")

	cmd := exec.Command(Binary, "-chdir="+opts.Dir, "show", "-json", PlanFile)
	cmd.Env = buildEnv(opts)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("terraform show -json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(opts.Dir, outFile), out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outFile, err)
	}
	return nil
}

// buildEnv returns the process environment with wrapper settings applied.
// Explicit TF_VAR_* values already in the environment are never overridden.
func buildEnv(opts *Options) []string {
	env := map[string]string{}
	order := []string{}
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		if _, seen := env[k]; !seen {
			order = append(order, k)
		}
		env[k] = v
	}
	set := func(k, v string) {
		if _, seen := env[k]; !seen {
			order = append(order, k)
		}
		env[k] = v
	}

	if d := opts.DataDir(); d != "" {
		set("TF_DATA_DIR", d)
	}
	if opts.AWSProfile != "" {
		set("AWS_PROFILE", opts.AWSProfile)
		set("AWS_DEFAULT_PROFILE", opts.AWSProfile)
	}
	if opts.VaultAddr != "" {
		set("VAULT_ADDR", opts.VaultAddr)
	}
	if strings.EqualFold(strings.TrimSpace(env["CI"]), "true") {
		if _, ok := env["TF_IN_AUTOMATION"]; !ok {
			set("TF_IN_AUTOMATION", "1")
		}
	}

	vars := opts.Context.TFVars()
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, ok := env[k]; !ok {
			set(k, vars[k])
		}
	}

	out := make([]string, 0, len(order))
	for _, k := range order {
		out = append(out, k+"="+env[k])
	}
	return out
}

// CleanCache removes the terraform data directory, dependency lock file and
// (unless SavePlan is set) the saved plan from the service directory.
func CleanCache(opts *Options) {
	_ = os.RemoveAll(filepath.Join(opts.Dir, ".terraform"))
	if d := opts.DataDir(); d != "" {
		_ = os.RemoveAll(filepath.Join(opts.Dir, d))
	}
	_ = os.Remove(filepath.Join(opts.Dir, ".terraform.lock.hcl"))
	if !opts.SavePlan {
		_ = os.Remove(filepath.Join(opts.Dir, PlanFile))
	}
}
