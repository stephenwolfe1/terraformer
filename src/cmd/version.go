package terraformer

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the client version information",
	Long: `Show the version for terraformer.

This will print a representation the version of terraformer.
The output will look something like this:

terraformer.BuildInfo{Version:"v1.0.0", GitCommit:"abc123", GitTreeState:"clean", GoVersion:"go1.16"}

- Version is the semantic version of the release.
- GitCommit is the SHA for the commit that this version was built from.
- GitTreeState is "clean" if there are no local code changes when this binary was
  built, and "dirty" if the binary was built from locally modified code.
-GoVersion is the version of Go used to build the binary.`,
	Run: doVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

var (
	// version is the current version of the Helm.
	// Update this whenever making a new release.
	// The version is of the format Major.Minor.Patch[-Prerelease][+BuildMetadata]
	//
	// Increment major number for new feature additions and behavioral changes.
	// Increment minor number for bug fixes and performance enhancements.
	// Increment patch number for critical fixes to existing releases.
	version = "v1.0"

	// metadata is extra build time data
	metadata = "unreleased"
	// gitCommit is the git sha1
	gitCommit = ""
	// gitTreeState is the state of the git tree
	gitTreeState = ""
)

// BuildInfo describes the compile time information.
type BuildInfo struct {
	// Version is the current semver.
	Version string `json:"version,omitempty"`
	// GitCommit is the git sha1.
	GitCommit string `json:"git_commit,omitempty"`
	// GitTreeState is the state of the git tree.
	GitTreeState string `json:"git_tree_state,omitempty"`
	// GoVersion is the version of the Go compiler used.
	GoVersion string `json:"go_version,omitempty"`
}

// getVersion returns the semver string of the version
func getVersion() string {
	if metadata == "" {
		return version
	}
	return version + "+" + metadata
}

func doVersion(cmd *cobra.Command, args []string) {
	v := BuildInfo{
		Version:      getVersion(),
		GitCommit:    gitCommit,
		GitTreeState: gitTreeState,
		GoVersion:    runtime.Version(),
	}

	fmt.Printf("%#v\n", v)
}
