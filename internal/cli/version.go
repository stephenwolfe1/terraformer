package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stephenwolfe1/terraformer/internal/terraform"
)

// Set at build time with -ldflags "-X github.com/stephenwolfe1/terraformer/internal/cli.version=..."
var (
	version      = "dev"
	gitCommit    = ""
	gitTreeState = ""
)

// BuildInfo describes the binary and the bundled terraform.
type BuildInfo struct {
	Version          string `json:"version"`
	GitCommit        string `json:"git_commit,omitempty"`
	GitTreeState     string `json:"git_tree_state,omitempty"`
	GoVersion        string `json:"go_version"`
	Platform         string `json:"platform"`
	TerraformVersion string `json:"terraform_version,omitempty"`
}

var versionJSON bool

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print terraformer and terraform version information",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		info := BuildInfo{
			Version:          version,
			GitCommit:        gitCommit,
			GitTreeState:     gitTreeState,
			GoVersion:        runtime.Version(),
			Platform:         runtime.GOOS + "/" + runtime.GOARCH,
			TerraformVersion: terraformVersion(),
		}
		if versionJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(info)
		}
		fmt.Printf("terraformer %s", info.Version)
		if info.GitCommit != "" {
			fmt.Printf(" (%s, %s)", info.GitCommit, info.GitTreeState)
		}
		fmt.Printf(" %s %s\n", info.GoVersion, info.Platform)
		if info.TerraformVersion != "" {
			fmt.Printf("terraform %s\n", info.TerraformVersion)
		}
		return nil
	},
}

func terraformVersion() string {
	out, err := exec.Command(terraform.Binary, "version", "-json").Output()
	if err != nil {
		return ""
	}
	var v struct {
		TerraformVersion string `json:"terraform_version"`
	}
	if json.Unmarshal(out, &v) != nil {
		return strings.TrimSpace(string(out))
	}
	return v.TerraformVersion
}

func init() {
	versionCmd.Flags().BoolVar(&versionJSON, "json", false, "print as JSON")
	rootCmd.AddCommand(versionCmd)
}
