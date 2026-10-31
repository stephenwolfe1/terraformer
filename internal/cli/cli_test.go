package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/stephenwolfe1/terraformer/internal/terraform"
)

var ctx = terraform.Context{Account: "prod", Environment: "staging", Region: "us-west-2"}

const enabledYAML = `
prod:
  staging:
    us-west-2:
      enabled: true
`

func serviceDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCheckServiceDir(t *testing.T) {
	g := gate{stateFile: "state.yaml"}
	tests := []struct {
		name     string
		files    map[string]string
		ctx      terraform.Context
		gate     gate
		wantSkip bool
		wantErr  bool
	}{
		{"enabled", map[string]string{"main.tf": "", "state.yaml": enabledYAML}, ctx, g, false, false},
		{"tf.json only", map[string]string{"main.tf.json": "{}"}, ctx, g, false, false},
		{"disabled", map[string]string{"main.tf": "", "state.yaml": strings.Replace(enabledYAML, "true", "false", 1)}, ctx, g, true, false},
		{"no terraform files", map[string]string{"state.yaml": enabledYAML}, ctx, g, true, false},
		{"no state file runs by default", map[string]string{"main.tf": ""}, ctx, g, false, false},
		{"no state file skipped when required", map[string]string{"main.tf": ""}, ctx, gate{stateFile: "state.yaml", require: true}, true, false},
		{"gating disabled ignores state file", map[string]string{"main.tf": "", "state.yaml": "not: [valid"}, terraform.Context{}, gate{}, false, false},
		{"account missing from state", map[string]string{"main.tf": "", "state.yaml": strings.Replace(enabledYAML, "prod:", "dev:", 1)}, ctx, g, true, false},
		{"environment missing from state", map[string]string{"main.tf": "", "state.yaml": strings.Replace(enabledYAML, "staging:", "production:", 1)}, ctx, g, true, false},
		{"region missing from state", map[string]string{"main.tf": "", "state.yaml": strings.Replace(enabledYAML, "us-west-2:", "eu-west-1:", 1)}, ctx, g, true, false},
		{"custom state filename", map[string]string{"main.tf": "", "enabled.yml": enabledYAML}, ctx, gate{stateFile: "enabled.yml", require: true}, false, false},
		{"invalid yaml", map[string]string{"main.tf": "", "state.yaml": "not: [valid: yaml"}, ctx, g, false, true},
		{"state file but no context", map[string]string{"main.tf": "", "state.yaml": enabledYAML}, terraform.Context{}, g, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := serviceDir(t, tt.files)
			skip, msg, err := checkServiceDir(dir, tt.ctx, tt.gate)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if skip != tt.wantSkip {
				t.Errorf("skip = %v, want %v (msg=%q)", skip, tt.wantSkip, msg)
			}
			if skip && msg == "" {
				t.Error("expected a message when skipping")
			}
		})
	}
}

func withConfig(t *testing.T, kv map[string]string) {
	t.Helper()
	for _, k := range []string{"services", "services-dir", "directory"} {
		viper.Set(k, "")
	}
	viper.Set("services-dir", "services")
	for k, v := range kv {
		viper.Set(k, v)
	}
	t.Cleanup(func() {
		for _, k := range []string{"services", "services-dir", "directory"} {
			viper.Set(k, nil)
		}
	})
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

func TestResolveTargets(t *testing.T) {
	base := t.TempDir()
	for _, d := range []string{"services/vpc", "services/rds", "infra/app", "other"} {
		os.MkdirAll(filepath.Join(base, d), 0o755)
	}
	os.WriteFile(filepath.Join(base, "file.txt"), nil, 0o644)
	chdir(t, base)

	rels := func(ts []target) []string {
		out := []string{}
		for _, x := range ts {
			out = append(out, x.rel)
		}
		return out
	}

	t.Run("path arg wins", func(t *testing.T) {
		withConfig(t, map[string]string{"services": "vpc"})
		ts, err := resolveTargets([]string{"other"})
		if err != nil || strings.Join(rels(ts), ",") != "other" {
			t.Fatalf("got %v, %v", rels(ts), err)
		}
		if !filepath.IsAbs(ts[0].abs) {
			t.Errorf("abs path not absolute: %q", ts[0].abs)
		}
	})
	t.Run("services env", func(t *testing.T) {
		withConfig(t, map[string]string{"services": " vpc  rds "})
		ts, err := resolveTargets(nil)
		if err != nil || strings.Join(rels(ts), ",") != "services/vpc,services/rds" {
			t.Fatalf("got %v, %v", rels(ts), err)
		}
	})
	t.Run("custom services dir", func(t *testing.T) {
		withConfig(t, map[string]string{"services": "app", "services-dir": "infra"})
		ts, err := resolveTargets(nil)
		if err != nil || strings.Join(rels(ts), ",") != "infra/app" {
			t.Fatalf("got %v, %v", rels(ts), err)
		}
	})
	t.Run("directory fallback", func(t *testing.T) {
		withConfig(t, map[string]string{"directory": "other"})
		ts, err := resolveTargets(nil)
		if err != nil || strings.Join(rels(ts), ",") != "other" {
			t.Fatalf("got %v, %v", rels(ts), err)
		}
	})
	t.Run("cwd fallback", func(t *testing.T) {
		withConfig(t, nil)
		ts, err := resolveTargets(nil)
		if err != nil || len(ts) != 1 || ts[0].rel != "." {
			t.Fatalf("got %v, %v", rels(ts), err)
		}
	})
	t.Run("missing service errors", func(t *testing.T) {
		withConfig(t, map[string]string{"services": "vpc nope"})
		if _, err := resolveTargets(nil); err == nil {
			t.Fatal("expected error for missing service")
		}
	})
	t.Run("file is not a directory", func(t *testing.T) {
		withConfig(t, nil)
		if _, err := resolveTargets([]string{"file.txt"}); err == nil {
			t.Fatal("expected error for file path")
		}
	})
}

func TestPathArgsAndSplit(t *testing.T) {
	cmd := subcommand("plan", "test")
	if err := cmd.ParseFlags([]string{"services/vpc", "--", "-target=module.x", "-refresh=false"}); err != nil {
		t.Fatal(err)
	}
	args := cmd.Flags().Args()
	if err := pathArgs(cmd, args); err != nil {
		t.Fatalf("pathArgs: %v", err)
	}
	pos, pass := splitArgs(cmd, args)
	if strings.Join(pos, ",") != "services/vpc" || strings.Join(pass, ",") != "-target=module.x,-refresh=false" {
		t.Errorf("split = %v | %v", pos, pass)
	}

	cmd2 := subcommand("plan", "test")
	_ = cmd2.ParseFlags([]string{"a", "b"})
	if err := pathArgs(cmd2, cmd2.Flags().Args()); err == nil {
		t.Error("expected error for two positional paths")
	}
}

func TestEnvBinding(t *testing.T) {
	t.Setenv("TERRAFORMER_AUTO_APPROVE", "true")
	t.Setenv("TERRAFORMER_EXTRA_ARGS", "-parallelism=3")
	t.Setenv("ENVIRONMENT", "dev")
	t.Setenv("SERVICES_DIR", "stacks")

	if !viper.GetBool("auto-approve") {
		t.Error("TERRAFORMER_AUTO_APPROVE not bound")
	}
	if viper.GetString("environment") != "dev" {
		t.Error("ENVIRONMENT not bound")
	}
	if viper.GetString("services-dir") != "stacks" {
		t.Error("SERVICES_DIR not bound")
	}

	t.Setenv("TERRAFORMER_ENVIRONMENT", "prod")
	if viper.GetString("environment") != "prod" {
		t.Error("TERRAFORMER_ENVIRONMENT should take precedence over ENVIRONMENT")
	}

	o := optionsFor(target{abs: "/x", rel: "x"}, []string{"-target=a"})
	if strings.Join(o.ExtraArgs, " ") != "-parallelism=3 -target=a" {
		t.Errorf("ExtraArgs = %v", o.ExtraArgs)
	}
	if !o.AutoApprove {
		t.Error("AutoApprove not propagated")
	}
	if !o.Upgrade {
		t.Error("Upgrade should default to true")
	}
	t.Setenv("TERRAFORMER_UPGRADE", "false")
	if optionsFor(target{abs: "/x", rel: "x"}, nil).Upgrade {
		t.Error("TERRAFORMER_UPGRADE=false not honored")
	}
}

func TestIsChangesPresent(t *testing.T) {
	exit2 := exec.Command("sh", "-c", "exit 2").Run()
	exit1 := exec.Command("sh", "-c", "exit 1").Run()
	de := []string{"-detailed-exitcode"}
	tests := []struct {
		name   string
		subcmd string
		args   []string
		err    error
		want   bool
	}{
		{"plan exit 2 detailed", "plan", de, exit2, true},
		{"plan exit 2 wrapped", "plan", de, fmt.Errorf("wrap: %w", exit2), true},
		{"plan exit 2 without flag", "plan", nil, exit2, false},
		{"plan exit 1", "plan", de, exit1, false},
		{"apply exit 2", "apply", de, exit2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isChangesPresent(tt.subcmd, tt.args, tt.err); got != tt.want {
				t.Errorf("isChangesPresent() = %v, want %v", got, tt.want)
			}
		})
	}
}
