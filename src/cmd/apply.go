package terraformer

import (
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// applyCmd represents the terraform apply command
var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "applys terraform code",
	Long: `Runs terraform apply.`,
	Run: runapply,
}

func init() {
	rootCmd.AddCommand(applyCmd)
}

func runapply(*cobra.Command, []string) {
	args := append([]string{"-chdir="+viper.GetString("directory"), "apply", "-auto-approve=true", "-lock=true", "-lock-timeout=0s", "-refresh=true"}, defaultOptions...)
	if viper.GetString("global-options") != "" {
		args = append([]string{viper.GetString("global-options")}, args...)
	}
	if viper.GetString("options") != "" {
		args = append(args, viper.GetString("options"))
	}

	cmd := exec.Command("terraform", args...)
	cmd.Env = append(os.Environ(),
		"TF_DATA_DIR="+os.ExpandEnv("${PWD}")+"/"+viper.GetString("data-directory"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		log.Error(err)
	}
}
