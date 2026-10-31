package terraform

import (
	"strings"
)

// Context is the optional deployment context a run is scoped to. Every field
// is optional; when set, the values are used to:
//
//   - isolate TF_DATA_DIR per context (.terraform-<account>-<environment>-<region>-<service>)
//   - name the saved plan JSON file
//   - expand placeholders in the backend key template
//   - export TF_VAR_account / TF_VAR_environment / TF_VAR_region / TF_VAR_repo_name
//   - gate services via the per-service state file (see cli.checkServiceDir)
type Context struct {
	Repo        string
	Account     string
	Environment string
	Region      string
}

// IsZero reports whether no context values are set.
func (c Context) IsZero() bool {
	return c.Repo == "" && c.Account == "" && c.Environment == "" && c.Region == ""
}

// slugService turns a service path into a filesystem-safe token.
func slugService(service string) string {
	s := strings.Trim(strings.ReplaceAll(service, "/", "-"), "-.")
	if s == "" {
		return "default"
	}
	return s
}

// Slug returns "<account>-<environment>-<region>-<service>", omitting empty
// context parts. It is used for TF_DATA_DIR and plan JSON names.
func (c Context) Slug(service string) string {
	parts := make([]string, 0, 4)
	for _, p := range []string{c.Account, c.Environment, c.Region} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	parts = append(parts, slugService(service))
	return strings.Join(parts, "-")
}

// Expand replaces {repo}, {service}, {account}, {environment} and {region}
// placeholders in tmpl.
func (c Context) Expand(tmpl, service string) string {
	return strings.NewReplacer(
		"{repo}", c.Repo,
		"{service}", strings.Trim(service, "/"),
		"{account}", c.Account,
		"{environment}", c.Environment,
		"{region}", c.Region,
	).Replace(tmpl)
}

// TFVars returns TF_VAR_* assignments for the non-empty context values.
// Environment variables are used (rather than -var flags) because Terraform
// silently ignores TF_VAR_* values for variables a configuration does not
// declare, so the wrapper works with any configuration.
func (c Context) TFVars() map[string]string {
	vars := map[string]string{}
	for name, v := range map[string]string{
		"account":     c.Account,
		"environment": c.Environment,
		"region":      c.Region,
		"repo_name":   c.Repo,
	} {
		if v != "" {
			vars["TF_VAR_"+name] = v
		}
	}
	return vars
}
