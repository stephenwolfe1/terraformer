package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"github.com/stephenwolfe1/terraformer/internal/terraform"
)

// stateFile is the per-service enablement file:
//
//	<account>:
//	  <environment>:
//	    <region>:
//	      enabled: true
type stateFile map[string]map[string]map[string]struct {
	Enabled bool `yaml:"enabled"`
}

// gate holds the service-gating configuration.
type gate struct {
	stateFile string // filename inside the service dir; "" disables gating
	require   bool   // skip services with no state file
}

// checkServiceDir decides whether terraform should run in dir.
// It returns skip=true with a message when the service should be skipped, or
// an error when the directory or state file is invalid.
//
//   - A directory without any *.tf / *.tf.json files is skipped.
//   - When g.stateFile is set and present, the service runs only if it is
//     enabled for the current account/environment/region.
//   - When the state file is missing, the service runs unless g.require is set.
func checkServiceDir(dir string, ctx terraform.Context, g gate) (skip bool, msg string, err error) {
	service := filepath.Base(dir)
	where := fmt.Sprintf("service=%s account=%s environment=%s region=%s", service, ctx.Account, ctx.Environment, ctx.Region)

	hasTF, err := containsTerraform(dir)
	if err != nil {
		return false, "", err
	}
	if !hasTF {
		return true, "no terraform files found, skipping  " + where, nil
	}

	if g.stateFile == "" {
		return false, "", nil
	}

	data, err := os.ReadFile(filepath.Join(dir, g.stateFile))
	if errors.Is(err, os.ErrNotExist) {
		if g.require {
			return true, g.stateFile + " missing, skipping  " + where, nil
		}
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("reading %s: %w", g.stateFile, err)
	}

	if ctx.Account == "" || ctx.Environment == "" || ctx.Region == "" {
		return false, "", fmt.Errorf("%s/%s exists but ACCOUNT, ENVIRONMENT and REGION are not all set (set them, or disable gating with --state-file='')", service, g.stateFile)
	}

	var state stateFile
	if err := yaml.Unmarshal(data, &state); err != nil {
		return false, "", fmt.Errorf("parsing %s: %w", g.stateFile, err)
	}

	if cfg, ok := state[ctx.Account][ctx.Environment][ctx.Region]; !ok || !cfg.Enabled {
		return true, fmt.Sprintf("service not enabled for %s/%s/%s, skipping  %s", ctx.Account, ctx.Environment, ctx.Region, where), nil
	}
	return false, "", nil
}

func containsTerraform(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if m, _ := filepath.Match("*.tf", e.Name()); m {
			return true, nil
		}
		if m, _ := filepath.Match("*.tf.json", e.Name()); m {
			return true, nil
		}
	}
	return false, nil
}
