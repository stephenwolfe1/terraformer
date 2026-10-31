package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// target is one service directory to run terraform in.
type target struct {
	abs string // absolute path passed to -chdir
	rel string // path as given (used for display, slugs and backend key)
}

// splitArgs separates positional args from terraform passthrough args given
// after "--" (e.g. `terraformer plan services/vpc -- -target=module.x`).
func splitArgs(cmd *cobra.Command, args []string) (positional, passthrough []string) {
	if dash := cmd.ArgsLenAtDash(); dash >= 0 {
		return args[:dash], args[dash:]
	}
	return args, nil
}

// pathArgs is the cobra Args validator: at most one positional path before "--".
func pathArgs(cmd *cobra.Command, args []string) error {
	positional, _ := splitArgs(cmd, args)
	if len(positional) > 1 {
		return fmt.Errorf("accepts at most one path, got %d (pass terraform flags after --)", len(positional))
	}
	return nil
}

// resolveTargets decides which directories to run in, in priority order:
//
//  1. an explicit path argument
//  2. SERVICES (space-separated names, resolved under SERVICES_DIR)
//  3. DIRECTORY
//  4. the current working directory
func resolveTargets(positional []string) ([]target, error) {
	if len(positional) == 1 {
		t, err := newTarget(positional[0])
		if err != nil {
			return nil, err
		}
		return []target{t}, nil
	}

	if names := strings.Fields(viper.GetString("services")); len(names) > 0 {
		dir := viper.GetString("services-dir")
		targets := make([]target, 0, len(names))
		for _, name := range names {
			t, err := newTarget(filepath.Join(dir, name))
			if err != nil {
				return nil, err
			}
			targets = append(targets, t)
		}
		return targets, nil
	}

	if d := viper.GetString("directory"); d != "" {
		t, err := newTarget(d)
		if err != nil {
			return nil, err
		}
		return []target{t}, nil
	}

	t, err := newTarget(".")
	if err != nil {
		return nil, err
	}
	return []target{t}, nil
}

func newTarget(path string) (target, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return target{}, fmt.Errorf("resolve path %q: %w", path, err)
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return target{}, fmt.Errorf("path %q does not exist: %w", path, err)
	}
	if !fi.IsDir() {
		return target{}, fmt.Errorf("path %q is not a directory", path)
	}
	return target{abs: abs, rel: filepath.Clean(path)}, nil
}
