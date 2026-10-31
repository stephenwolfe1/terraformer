package terraform

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var fullCtx = Context{Repo: "my-infra", Account: "prod", Environment: "staging", Region: "us-west-2"}

func TestContextSlug(t *testing.T) {
	tests := []struct {
		name    string
		ctx     Context
		service string
		want    string
	}{
		{"full", fullCtx, "services/vpc", "prod-staging-us-west-2-services-vpc"},
		{"nested", Context{Account: "dev", Environment: "test", Region: "eu-west-1"}, "nested/deep/svc", "dev-test-eu-west-1-nested-deep-svc"},
		{"empty service", fullCtx, "", "prod-staging-us-west-2-default"},
		{"dot service", fullCtx, ".", "prod-staging-us-west-2-default"},
		{"partial context", Context{Environment: "dev"}, "svc", "dev-svc"},
		{"no context", Context{}, "services/vpc", "services-vpc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ctx.Slug(tt.service); got != tt.want {
				t.Errorf("Slug(%q) = %q, want %q", tt.service, got, tt.want)
			}
		})
	}
}

func TestContextExpand(t *testing.T) {
	got := fullCtx.Expand("{repo}/{service}/{account}/{environment}/{region}/terraform.tfstate", "services/vpc/")
	want := "my-infra/services/vpc/prod/staging/us-west-2/terraform.tfstate"
	if got != want {
		t.Errorf("Expand() = %q, want %q", got, want)
	}
	if got := (Context{}).Expand("static/key", "svc"); got != "static/key" {
		t.Errorf("Expand() without placeholders = %q", got)
	}
}

func TestContextTFVars(t *testing.T) {
	got := Context{Account: "a", Region: "r"}.TFVars()
	want := map[string]string{"TF_VAR_account": "a", "TF_VAR_region": "r"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TFVars() = %v, want %v", got, want)
	}
	if len((Context{}).TFVars()) != 0 {
		t.Error("expected no TF_VARs for empty context")
	}
}

func TestDataDir(t *testing.T) {
	t.Setenv("TF_DATA_DIR", "")
	os.Unsetenv("TF_DATA_DIR")

	o := &Options{Service: "services/vpc", Context: fullCtx}
	if got, want := o.DataDir(), ".terraform-prod-staging-us-west-2-services-vpc"; got != want {
		t.Errorf("DataDir() = %q, want %q", got, want)
	}
	if got := (&Options{Service: "svc"}).DataDir(); got != "" {
		t.Errorf("DataDir() without context = %q, want empty", got)
	}

	t.Setenv("TF_DATA_DIR", "/custom")
	if got := o.DataDir(); got != "" {
		t.Errorf("DataDir() with explicit TF_DATA_DIR = %q, want empty (respect user)", got)
	}
}

func TestPlanJSONFilename(t *testing.T) {
	o := &Options{Service: "services/vpc", Context: fullCtx}
	if got, want := o.PlanJSONFilename(), "prod-staging-us-west-2-services-vpc.json"; got != want {
		t.Errorf("PlanJSONFilename() = %q, want %q", got, want)
	}
	o.PlanJSONFile = "plan.json"
	if got := o.PlanJSONFilename(); got != "plan.json" {
		t.Errorf("override ignored: %q", got)
	}
}

func TestInitArgs(t *testing.T) {
	o := &Options{Service: "services/vpc", Context: fullCtx}
	if got, want := initArgs(o, true), []string{"init", "-input=false", "-reconfigure"}; !reflect.DeepEqual(got, want) {
		t.Errorf("initArgs no key = %v, want %v", got, want)
	}

	o.BackendKey = "{repo}/{service}/{account}/{environment}/{region}/terraform.tfstate"
	got := initArgs(o, true)
	if last := got[len(got)-1]; last != "-backend-config=key=my-infra/services/vpc/prod/staging/us-west-2/terraform.tfstate" {
		t.Errorf("backend key arg = %q", last)
	}

	if got, want := initArgs(o, false), []string{"init", "-input=false", "-backend=false"}; !reflect.DeepEqual(got, want) {
		t.Errorf("initArgs without backend = %v, want %v", got, want)
	}

	o.Upgrade, o.BackendKey = true, ""
	if got, want := initArgs(o, true), []string{"init", "-input=false", "-upgrade", "-reconfigure"}; !reflect.DeepEqual(got, want) {
		t.Errorf("initArgs upgrade = %v, want %v", got, want)
	}
	if got, want := initArgs(o, false), []string{"init", "-input=false", "-upgrade", "-backend=false"}; !reflect.DeepEqual(got, want) {
		t.Errorf("initArgs upgrade without backend = %v, want %v", got, want)
	}
}

func TestSubcommandArgs(t *testing.T) {
	tests := []struct {
		name     string
		opts     Options
		subcmd   string
		want     []string
		wantMode OutputMode
	}{
		{"plan", Options{}, "plan", []string{"plan", "-input=false", "-lock=false"}, OutputPlanFilter},
		{"plan save", Options{SavePlan: true}, "plan", []string{"plan", "-input=false", "-lock=false", "-out=terraform.tfplan"}, OutputPlanFilter},
		{"apply interactive", Options{}, "apply", []string{"apply", "-lock=true", "-lock-timeout=0s"}, OutputNormal},
		{"apply auto", Options{AutoApprove: true}, "apply", []string{"apply", "-lock=true", "-lock-timeout=0s", "-input=false", "-auto-approve"}, OutputNormal},
		{"destroy ignores auto-approve", Options{AutoApprove: true}, "destroy", []string{"destroy", "-lock=true", "-lock-timeout=0s"}, OutputNormal},
		{"extra args last", Options{ExtraArgs: []string{"-lock=true", "-target=module.x"}}, "plan", []string{"plan", "-input=false", "-lock=false", "-lock=true", "-target=module.x"}, OutputPlanFilter},
		{"validate", Options{}, "validate", []string{"validate"}, OutputNormal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, mode := subcommandArgs(&tt.opts, tt.subcmd)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("args = %v, want %v", got, tt.want)
			}
			if mode != tt.wantMode {
				t.Errorf("mode = %v, want %v", mode, tt.wantMode)
			}
		})
	}
}

func envMap(env []string) map[string]string {
	m := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		m[k] = v
	}
	return m
}

func TestBuildEnv(t *testing.T) {
	os.Unsetenv("TF_DATA_DIR")
	t.Setenv("TF_VAR_region", "explicit")
	t.Setenv("CI", "true")
	t.Setenv("AWS_PROFILE", "old")

	env := envMap(buildEnv(&Options{
		Service:    "services/vpc",
		Context:    fullCtx,
		AWSProfile: "new",
		VaultAddr:  "https://vault.example.com",
	}))

	checks := map[string]string{
		"TF_DATA_DIR":         ".terraform-prod-staging-us-west-2-services-vpc",
		"AWS_PROFILE":         "new",
		"AWS_DEFAULT_PROFILE": "new",
		"VAULT_ADDR":          "https://vault.example.com",
		"TF_VAR_account":      "prod",
		"TF_VAR_environment":  "staging",
		"TF_VAR_repo_name":    "my-infra",
		"TF_VAR_region":       "explicit", // user value wins
		"TF_IN_AUTOMATION":    "1",
	}
	for k, want := range checks {
		if got := env[k]; got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestBuildEnv_noOverridesWhenUnset(t *testing.T) {
	os.Unsetenv("TF_DATA_DIR")
	t.Setenv("AWS_PROFILE", "keep")
	t.Setenv("CI", "")
	env := envMap(buildEnv(&Options{Service: "svc"}))
	if env["AWS_PROFILE"] != "keep" {
		t.Errorf("AWS_PROFILE overridden: %q", env["AWS_PROFILE"])
	}
	if _, ok := env["TF_DATA_DIR"]; ok {
		t.Error("TF_DATA_DIR should not be set without context")
	}
	if _, ok := env["TF_IN_AUTOMATION"]; ok {
		t.Error("TF_IN_AUTOMATION should not be set outside CI")
	}
}

func TestCleanCache(t *testing.T) {
	os.Unsetenv("TF_DATA_DIR")
	dir := t.TempDir()
	o := &Options{Dir: dir, Service: "svc", Context: Context{Environment: "dev"}}
	for _, p := range []string{".terraform/x", ".terraform-dev-svc/x"} {
		os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0o755)
		os.WriteFile(filepath.Join(dir, p), nil, 0o644)
	}
	os.WriteFile(filepath.Join(dir, ".terraform.lock.hcl"), nil, 0o644)
	os.WriteFile(filepath.Join(dir, PlanFile), nil, 0o644)
	os.WriteFile(filepath.Join(dir, "main.tf"), nil, 0o644)

	o.SavePlan = true
	CleanCache(o)
	for _, gone := range []string{".terraform", ".terraform-dev-svc", ".terraform.lock.hcl"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("%s should be removed", gone)
		}
	}
	for _, kept := range []string{PlanFile, "main.tf"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s should be kept: %v", kept, err)
		}
	}

	o.SavePlan = false
	CleanCache(o)
	if _, err := os.Stat(filepath.Join(dir, PlanFile)); !os.IsNotExist(err) {
		t.Error("plan file should be removed when SavePlan is false")
	}
}
